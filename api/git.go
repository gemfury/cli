package api

import (
	"context"
	"io"
	"net/url"
)

// GitList returns a listing of Git repositories for an account
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
	ID    string `json:"id"`
	Name  string `json:"name"`
	Stack struct {
		Name string `json:"name"`
	} `json:"build_stack"`
}

// GitInfo returns the details of a specific Git repository
func (c *Client) GitInfo(cc context.Context, repo string) (*GitRepo, error) {
	path := "/git/repos/{acct}/" + url.PathEscape(repo)
	req := c.newRequest(cc, "GET", path, false)

	resp := gitInfoResponse{}
	err := req.doJSON(&resp)
	return &resp.Repo, err
}

// gitInfoResponse represents details from Git Info API call
type gitInfoResponse struct {
	Repo GitRepo `json:"repo"`
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
func (c *Client) GitRebuild(cc context.Context, out io.Writer, repo, revision string) error {
	path := "/git/repos/{acct}/" + url.PathEscape(repo) + "/builds"
	if revision != "" {
		path = path + "?build[revision]=" + url.QueryEscape(revision)
	}
	req := c.newRequest(cc, "POST", path, false)
	return req.doWithOutput(out)
}
