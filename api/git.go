package api

import (
	"context"
	"net/url"
)

// GitReset removes a Gemfury Git repository
func (c *Client) GitReset(cc context.Context, repo string) error {
	path := "/git/repos/{acct}/" + url.PathEscape(repo)
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
