package api

import (
	"context"
)

// Logout deletes the CLI token on the server
func (c *Client) Logout(cc context.Context) error {
	req := c.newRequest(cc, "POST", "/logout", false)
	return req.doJSON(nil)
}

// Logout deletes the CLI token on the server
func (c *Client) Login(cc context.Context, loginReq *LoginRequest) (*LoginResponse, error) {
	req := c.newRequest(cc, "POST", "/login", false)

	if err := c.prepareJSONBody(req, loginReq); err != nil {
		return nil, err
	}

	resp := &LoginResponse{}
	err := req.doJSON(resp)
	return resp, err
}

// LoginRequest represents Login JSON
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents Login JSON
type LoginResponse struct {
	Token string          `json:"token"`
	User  AccountResponse `json:"user"`
}
