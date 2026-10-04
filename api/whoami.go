package api

import (
	"cmp"
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
	AccountBasic
	Type     string `json:"type"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// Login is what a session of the account is saved by, and known to Git
// as: its email, or its username for an organization, which has none
func (a AccountResponse) Login() string {
	return cmp.Or(a.Email, a.Username)
}

// AccountBasic represents the Account JSON fields common to every account
type AccountBasic struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
