package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
)

// Logout deletes the CLI token on the server
func (c *Client) Logout(cc context.Context) error {
	req := c.newRequest(cc, "POST", "/logout", false)
	return req.doJSON(nil)
}

// Logout deletes the CLI token on the server
func (c *Client) Login(cc context.Context, loginReq *LoginRequest) (*LoginResponse, error) {
	body, err := json.Marshal(loginReq)
	if err != nil {
		return nil, err
	}

	req := c.newRequest(cc, "POST", "/login", false)
	req.Header.Set("Content-Type", "application/json")
	req.Body = ioutil.NopCloser(bytes.NewReader(body))
	resp := &LoginResponse{}

	err = req.doJSON(resp)
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
