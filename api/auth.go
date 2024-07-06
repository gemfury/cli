package api

import (
	"context"
)

// Logout deletes the CLI token on the server
func (c *Client) Logout(cc context.Context) error {
	req := c.newRequest(cc, "POST", "/logout", false)
	return req.doJSON(nil)
}

// Interactive login generates the CLI token on the server from username/password
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

// LoginCreate generates an URL used to approve a CLI login via browser authentication
func (c *Client) LoginCreate(cc context.Context) (*LoginCreateResponse, error) {
	req := c.newRequest(cc, "POST", "/cli/auth", false)
	resp := &LoginCreateResponse{}
	err := req.doJSON(resp)
	return resp, err
}

// LoginCreateResponse represents LoginCreate JSON response
type LoginCreateResponse struct {
	BrowserURL string `json:"browser_url"`
	CLIURL     string `json:"cli_url"`
	Token      string `json:"token"`
}

// LoginGet waits for browser login and retrieves its results (token & user information)
func (c *Client) LoginGet(cc context.Context, create *LoginCreateResponse) (*LoginGetResponse, error) {
	req := c.newRequest(cc, "GET", create.CLIURL, false)
	req.Header.Set("Authorization", "Bearer "+create.Token)
	resp := &LoginGetResponse{}
	err := req.doJSON(resp)
	return resp, err
}

// LoginGetResponse represents LoginGet JSON response
type LoginGetResponse struct {
	Error string `json:"error"`
	LoginResponse
}
