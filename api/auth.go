package api

import (
	"context"
)

// Logout deletes the CLI token on the server
func (c *Client) Logout(cc context.Context) error {
	req := c.newRequest(cc, "POST", "/logout", false)
	return req.doJSON(nil)
}
