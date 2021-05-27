package api

import (
	"context"
	"net/url"
)

// Removes a package version from the current account
func (c *Client) Yank(cc context.Context, pkg, version string) error {
	p, v := url.PathEscape(pkg), url.PathEscape(version)
	req := c.newRequest(cc, "DELETE", "/packages/"+p+"/versions/"+v, true)
	return req.doJSON(nil)
}
