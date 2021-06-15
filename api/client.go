package api

import (
	"github.com/yosida95/uritemplate/v3"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

var (
	// ErrFuryServer is the error for 5xx server errors
	ErrFuryServer = errors.New("Fury-API server error")

	// ErrUnauthorized is the error for 401 from server
	ErrUnauthorized = errors.New("Authentication failure")

	// ErrForbidden is the error for 403 from server
	ErrForbidden = errors.New("You're not allowed to do this")

	// ErrNotFound is the error for 404 from server
	ErrNotFound = errors.New("Doesn't look like this exists")

	// Default "Accept" header for Gemfury API requests
	hdrAcceptAPIv1 = "application/vnd.fury.v1"

	// Default "User-Agent" header for Gemfury API requests
	hdrUserAgent = "Gemfury Go-CLI %s"

	// Default API endpoint
	defaultEndpoint = "https://api.fury.io"

	// DefaultConduit is a wrapper for http.DefaultClient
	DefaultConduit = &conduitStandard{
		Client:  http.DefaultClient,
		Version: "???",
	}
)

// StatusCodeToError converts API response status to error code
func StatusCodeToError(s int) error {
	switch {
	case s == 401:
		return ErrUnauthorized
	case s == 403:
		return ErrForbidden
	case s == 404:
		return ErrNotFound
	case s >= 200 && s < 300:
		return nil
	case s >= 500:
		return ErrFuryServer
	default:
		return fmt.Errorf(http.StatusText(s))
	}
}

// Client is the main entrypoint for interacting with Gemfury API
type Client struct {
	Version string
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
	return c.makeRequest(cc, method, defaultEndpoint, rawPath, impersonate)
}

func (c *Client) newPushRequest(cc context.Context, method, rawPath string, impersonate bool) *request {
	return c.makeRequest(cc, method, "https://push.fury.io", rawPath, impersonate)
}

func (c *Client) makeRequest(cc context.Context, method, base, rawPath string, impersonate bool) *request {
	baseURL, _ := url.Parse(base)

	// Render URI Templates (RFC6570) to populate {acct}, etc
	reqURL, err := c.renderURITemplate(baseURL.String() + rawPath)
	if err != nil {
		return &request{err: err}
	}

	// Append impersonation, if requested
	if as := c.Account; impersonate && as != "" {
		query := url.Values{"as": []string{as}}.Encode()
		if strings.Contains(reqURL, "?") {
			reqURL = reqURL + "&" + query
		} else {
			reqURL = reqURL + "?" + query
		}
	}

	// Generate http.Request object using conduit
	r, err := c.conduit.NewRequest(cc, method, reqURL, nil)

	// Populate authentication, if present
	if token := c.Token; r != nil && token != "" {
		r.Header.Set("Authorization", token)
	}

	return &request{Request: r, err: err, conduit: c.conduit}
}

// Populate API request body as JSON with the proper Content-Type header
func (c *Client) prepareJSONBody(req *request, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Body = ioutil.NopCloser(bytes.NewReader(body))
	return nil
}

// Use URI Templates (RFC6570) to generate templates
func (c *Client) renderURITemplate(pathTemplate string) (string, error) {
	tmpl, err := uritemplate.New(pathTemplate)
	if err != nil {
		return "", err
	}

	acct := c.Account
	if acct == "" {
		acct = "me"
	}

	return tmpl.Expand(uritemplate.Values{
		"acct": uritemplate.String(acct),
	})
}

// API Request to be executed on client
type request struct {
	*http.Request
	err error
	conduit
}

// Common request processing (standard semantics for closing resp.Body)
func (r *request) doCommon() (*http.Response, error) {
	if r.err != nil {
		return nil, r.err
	}

	resp, err := r.conduit.Do(r.Request)
	if err != nil {
		return resp, err
	}

	if err := StatusCodeToError(resp.StatusCode); err != nil {
		resp.Body.Close()
		return resp, err
	}

	return resp, nil
}

// Fetch and decode JSON from Gemfury with Authentication, returns error
func (r *request) doJSON(data interface{}) error {
	_, err := r.doPaginatedJSON(data)
	return err
}

// Fetch and decode JSON from Gemfury with Authentication, returns pagination and error
func (r *request) doPaginatedJSON(data interface{}) (*PaginationResponse, error) {
	resp, err := r.doCommon()
	if err != nil {
		r.err = err
		return nil, err
	}

	defer resp.Body.Close()

	// Parse pagination headers
	pagination := parsePagination(resp)

	// No body expected or no body given
	if resp.StatusCode == 204 || data == nil {
		return pagination, nil
	}

	// Decode body JSON into provided data structure
	r.err = json.NewDecoder(resp.Body).Decode(data)
	return pagination, r.err
}

// Create Gemfury API request, and then stream output
func (r *request) doWithOutput(out io.Writer) error {
	resp, err := r.doCommon()
	if err != nil {
		r.err = err
		return err
	}

	defer resp.Body.Close()

	_, r.err = io.Copy(out, resp.Body)
	return r.err
}

// Wrapper for net/http and http.Client
type conduit interface {
	NewRequest(context.Context, string, string, io.Reader) (*http.Request, error)
	Do(*http.Request) (*http.Response, error)
}

type conduitStandard struct {
	Version string
	*http.Client
}

func (c *conduitStandard) NewRequest(cc context.Context, url, method string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(cc, url, method, body)
	if err != nil {
		return req, err
	}

	req.Header.Set("User-Agent", fmt.Sprintf(hdrUserAgent, c.Version))
	req.Header.Add("Accept", hdrAcceptAPIv1)
	return req, nil
}
