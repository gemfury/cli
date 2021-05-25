package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

var (
	ErrFuryServer  = errors.New("Fury-API server error")
	ErrClientAuth  = errors.New("Unauthorized client")
	DefaultConduit = &conduitStandard{http.DefaultClient}
)

type Client struct {
	conduit conduit
	Account string
	Token   string
}

// Generate new client
func NewClient(token, account string) *Client {
	return &Client{
		conduit: DefaultConduit,
		Account: account,
		Token:   token,
	}
}

// Generate URL to access API
func (c *Client) urlFor(rawPath string, impersonate bool) string {
	baseURL, _ := url.Parse("https://api.fury.io")

	if token := c.Token; token != "" {
		baseURL.User = url.UserPassword(token, "")
	}

	out := baseURL.String()
	out = out + rawPath

	if as := c.Account; impersonate && as != "" {
		out = out + "?" + url.Values{"as": []string{as}}.Encode()
	}

	return out
}

// Fetch and decode JSON from Gemfury with Authentication, returns expiry and error
func (c *Client) doJSON(cc context.Context, method, url string, data interface{}) error {
	req, err := c.conduit.NewRequest(cc, method, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.conduit.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if err := StatusCodeToError(resp.StatusCode); err != nil {
		return err
	}

	if resp.StatusCode == 204 || data == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(data)
}

// Convert API response status to error code
func StatusCodeToError(s int) error {
	if s >= 200 && s < 300 {
		return nil
	} else if s >= 401 && s <= 403 {
		return ErrClientAuth
	} else if s >= 500 {
		return ErrFuryServer
	} else {
		return fmt.Errorf(http.StatusText(s))
	}
}

// Wrapper for net/http and http.Client
type conduit interface {
	NewRequest(context.Context, string, string, io.Reader) (*http.Request, error)
	Do(*http.Request) (*http.Response, error)
}

type conduitStandard struct {
	*http.Client
}

func (c *conduitStandard) NewRequest(cc context.Context, url, method string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(cc, url, method, body)
	if err != nil {
		return req, err
	}

	req.Header.Add("Accept", "application/json")
	return req, nil
}
