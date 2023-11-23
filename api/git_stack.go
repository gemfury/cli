package api

import (
	"context"
	"net/url"
)

// Packages returns the details of the package listing
func (c *Client) GitStacks(cc context.Context) ([]GitStack, error) {
	req := c.newRequest(cc, "GET", "/git/stacks", false)

	resp := []GitStack{}
	if err := req.doJSON(&resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// Repo represents Git Config KV pair
type GitStack struct {
	Name string
}

// GitStackSet updates stack for a Gemfury Git repository
func (c *Client) GitStackSet(cc context.Context, repo, newStack string) error {
	path := "/git/repos/{acct}/" + url.PathEscape(repo)
	path = path + "?repo[build_stack]=" + url.QueryEscape(newStack)
	req := c.newRequest(cc, "PATCH", path, false)
	return req.doJSON(nil)
}
