package api

import (
	"context"
)

// WhoAmI returns the details of the currently logged-in account
func (c *Client) WhoAmI(cc context.Context) (*WhoAmIResponse, error) {
	req := c.newRequest(cc, "GET", "/users/me", false)
	resp := &WhoAmIResponse{}

	err := req.doJSON(resp)
	return resp, err
}

// WhoAmIResponse represents Account JSON
type WhoAmIResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}
