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
	// ErrFuryServer is the error for 5xx server errors
	ErrFuryServer = errors.New("Fury-API server error")

	// ErrClientAuth is the error for 401/403 from server
	ErrClientAuth = errors.New("Unauthorized client")

	// DefaultConduit is a wrapper for http.DefaultClient
	DefaultConduit = &conduitStandard{http.DefaultClient}

	// Default "Accept" header for Gemfury API requests
	hdrAcceptAPIv1 = "application/vnd.fury.v1"
)

// StatusCodeToError converts API response status to error code
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

// Client is the main entrypoint for interacting with Gemfury API
type Client struct {
	conduit conduit
	Account string
	Token   string
}

// NewClient creates a new client using the DefaultConduit
func NewClient(token, account string) *Client {
	return &Client{
		conduit: DefaultConduit,
		Account: account,
		Token:   token,
	}
}

func (c *Client) newRequest(cc context.Context, method, rawPath string, impersonate bool) *request {
	return c.makeRequest(cc, method, "https://api.fury.io", rawPath, impersonate)
}

func (c *Client) newPushRequest(cc context.Context, method, rawPath string, impersonate bool) *request {
	return c.makeRequest(cc, method, "https://push.fury.io", rawPath, impersonate)
}

func (c *Client) makeRequest(cc context.Context, method, base, rawPath string, impersonate bool) *request {
	baseURL, _ := url.Parse(base)

	if token := c.Token; token != "" {
		baseURL.User = url.UserPassword(token, "")
	}

	out := baseURL.String()
	out = out + rawPath

	if as := c.Account; impersonate && as != "" {
		out = out + "?" + url.Values{"as": []string{as}}.Encode()
	}

	req, err := c.conduit.NewRequest(cc, method, out, nil)
	return &request{Request: req, err: err, conduit: c.conduit}
}

// API Request to be executed on client
type request struct {
	*http.Request
	err error
	conduit
}

// Fetch and decode JSON from Gemfury with Authentication, returns expiry and error
func (r *request) doJSON(data interface{}) error {
	if r.err != nil {
		return r.err
	}

	resp, err := r.conduit.Do(r.Request)
	if err != nil {
		r.err = err
		return err
	}

	defer resp.Body.Close()

	if err := StatusCodeToError(resp.StatusCode); err != nil {
		r.err = err
		return err
	}

	if resp.StatusCode == 204 || data == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(data)
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

	req.Header.Add("Accept", hdrAcceptAPIv1)
	return req, nil
}
