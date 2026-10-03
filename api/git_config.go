package api

import (
	"context"
	"net/url"
)

// GitConfig returns the build environment of a Git repository, by key
func (c *Client) GitConfig(cc context.Context, repo string) (map[string]string, error) {
	path := "/git/repos/{acct}/" + url.PathEscape(repo) + "/config-vars"
	req := c.newRequest(cc, "GET", path, false)

	resp := gitConfigJSON{}
	if err := req.doJSON(&resp); err != nil {
		return nil, err
	}

	out := make(map[string]string, len(resp.ConfigVars))
	for k, v := range resp.ConfigVars {
		if v != nil { // Should not happen
			out[k] = *v
		}
	}

	return out, nil
}

// Git Config request/response
type gitConfigJSON struct {
	ConfigVars map[string]*string `json:"config_vars"`
}

// GitConfigSet updates Git Config with passed-in map of new variables
func (c *Client) GitConfigSet(cc context.Context, repo string, vars map[string]*string) error {
	path := "/git/repos/{acct}/" + url.PathEscape(repo) + "/config-vars"
	req := c.newRequest(cc, "PATCH", path, false)
	if err := c.prepareJSONBody(req, &gitConfigJSON{vars}); err != nil {
		return err
	}
	return req.doJSON(nil)
}
