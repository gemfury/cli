package api

import (
	"context"
	"net/url"
)

// Packages returns the details of the package listing
func (c *Client) GitConfig(cc context.Context, repo string) ([]GitConfigPair, error) {
	path := "/git/repos/{acct}/" + url.PathEscape(repo) + "/config-vars"
	req := c.newRequest(cc, "GET", path, false)

	resp := gitConfigJSON{}
	if err := req.doJSON(&resp); err != nil {
		return nil, err
	}

	out := make([]GitConfigPair, 0, len(resp.ConfigVars))
	for k, v := range resp.ConfigVars {
		out = append(out, GitConfigPair{k, v})
	}

	return out, nil
}

// Git Config request/response
type gitConfigJSON struct {
	ConfigVars map[string]string `json:"config_vars"`
}

// Repo represents Git Config KV pair
type GitConfigPair struct {
	Key   string
	Value string
}

// GitConfigSet updates Git Config with passed-in map of new variables
func (c *Client) GitConfigSet(cc context.Context, repo string, vars map[string]string) error {
	path := "/git/repos/{acct}/" + url.PathEscape(repo) + "/config-vars"
	req := c.newRequest(cc, "PATCH", path, false)
	c.prepareJSONBody(req, &gitConfigJSON{vars})
	return req.doJSON(nil)
}
