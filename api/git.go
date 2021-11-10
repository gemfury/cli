package api

import (
	"context"
	"io"
	"net/url"
)

// Packages returns the details of the package listing
func (c *Client) GitList(cc context.Context, body *PaginationRequest) (*GitReposResponse, error) {
	req := c.newRequest(cc, "GET", "/git/repos/{acct}", false)

	if body != nil {
		c.prepareJSONBody(req, body)
	}

	resp := GitReposResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Root)
	resp.Pagination = pagination

	return &resp, err
}

// ReposResponse represents details from Git List API call
type GitReposResponse struct {
	Pagination *PaginationResponse
	Root       struct {
		Repos []*GitRepo
	}
}

// Repo represents Git Repo JSON
type GitRepo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GitDestroy either fully removes a Gemfury Git repository, or resets the repo
// by deleting content but keeping its history, configuraion, and related metadata.
func (c *Client) GitDestroy(cc context.Context, repo string, resetOnly bool) error {
	path := "/git/repos/{acct}/" + url.PathEscape(repo)

	// Resets Git repository content without destroying it
	// Keeping ID, build history, configuration, etc
	if resetOnly {
		path = path + "?reset=1"
	}

	req := c.newRequest(cc, "DELETE", path, false)
	return req.doJSON(nil)
}

// GitRename renames a Gemfury Git repository
func (c *Client) GitRename(cc context.Context, repo, newName string) error {
	path := "/git/repos/{acct}/" + url.PathEscape(repo)
	path = path + "?repo[name]=" + url.QueryEscape(newName)
	req := c.newRequest(cc, "PATCH", path, false)
	return req.doJSON(nil)
}

// GitRename renames a Gemfury Git repository
func (c *Client) GitRebuild(cc context.Context, out io.Writer, repo string) error {
	path := "/git/repos/{acct}/" + url.PathEscape(repo) + "/builds"
	req := c.newRequest(cc, "POST", path, false)
	return req.doWithOutput(out)
}
