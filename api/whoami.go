package api

import (
	"context"
)

// WhoAmI returns the details of the currently logged-in account
func (c *Client) WhoAmI(cc context.Context) (*AccountResponse, error) {
	req := c.newRequest(cc, "GET", "/users/me", false)
	resp := &AccountResponse{}

	err := req.doJSON(resp)
	return resp, err
}

// AccountResponse represents Account JSON
type AccountResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}
