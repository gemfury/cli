package api

import (
	"github.com/hashicorp/go-retryablehttp"
	"github.com/yosida95/uritemplate/v3"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// Default "Accept" header for Gemfury API requests
	hdrAcceptAPIv1 = "application/vnd.fury.v1"

	// Default "User-Agent" header for Gemfury API requests
	hdrUserAgent = "Gemfury CLI %s"

	// Response header with the ID of the request, to report a failure by
	hdrRequestID = "X-Request-Id"

	// Bounds of the waits between retries: see backoff
	retryWaitMin  = time.Second
	retryWaitMax  = 20 * time.Second
	retryAfterMax = time.Minute

	// Most of an unread response body to read, so as to reuse its connection
	maxDrainBytes = 64 << 10

	// Most of the body of a failed response to decode
	maxErrorBytes = 64 << 10
)

var (
	// DefaultConduit is how every client sends its requests: by the
	// transport of the process, with headers and retries.
	DefaultConduit = &conduitStandard{
		Transport: http.DefaultTransport,
		Version:   "???",
	}
)

// Client is the main entrypoint for interacting with Gemfury API
type Client struct {
	// Base URLs of uploads and of the rest of the API. Request paths
	// are joined below them, so they may have a path of their own.
	PushEndpoint *url.URL
	Endpoint     *url.URL

	Account string
	Token   string

	// Timeout of a request, and LongTimeout of one that transfers a
	// file or waits for a browser login
	Timeout     time.Duration
	LongTimeout time.Duration

	// Retries of a GET or HEAD that fails, with OnRetry told of each wait
	OnRetry func(string)
	Retries int
}

// NewClient creates a new client, which sends by DefaultConduit. Its
// endpoints are those of Gemfury, as URLs of its own that a caller may change.
func NewClient(token, account string) *Client {
	return &Client{
		PushEndpoint: &url.URL{Scheme: "https", Host: "push.fury.io"},
		Endpoint:     &url.URL{Scheme: "https", Host: "api.fury.io"},
		Account:      account,
		Token:        token,
		Timeout:      30 * time.Second,
		LongTimeout:  10 * time.Minute,
		Retries:      3,
	}
}

func (c *Client) newRequest(cc context.Context, method, rawPath string, impersonate bool) *request {
	req := c.makeRequest(cc, method, c.Endpoint, rawPath, impersonate)
	req.timeout = c.Timeout
	return req
}

// A download, or the wait for a browser login, is given as long as an upload
func (c *Client) newLongRequest(cc context.Context, method, rawPath string, impersonate bool) *request {
	req := c.newRequest(cc, method, rawPath, impersonate)
	req.timeout = c.LongTimeout
	return req
}

func (c *Client) newPushRequest(cc context.Context, method, rawPath string, impersonate bool) *request {
	req := c.makeRequest(cc, method, c.PushEndpoint, rawPath, impersonate)
	req.timeout = c.LongTimeout
	return req
}

func (c *Client) makeRequest(cc context.Context, method string, base *url.URL, rawPath string, impersonate bool) *request {
	reqURL, err := c.requestURL(base, rawPath, impersonate)
	if err != nil {
		return &request{err: err}
	}

	// Generate http.Request object using the conduit
	r, err := DefaultConduit.NewRequest(cc, method, reqURL, nil)

	// Populate authentication, if present
	if token := c.Token; r != nil && token != "" {
		r.Header.Set("Authorization", token)
	}

	return &request{Request: r, err: err, retries: c.Retries, onRetry: c.OnRetry}
}

// requestURL is rawPath, with its query if it has one, below base, which
// may have a path of its own. URI Templates (RFC6570) in the path are
// rendered to populate {acct}, etc, and impersonation is added to the query.
func (c *Client) requestURL(base *url.URL, rawPath string, impersonate bool) (string, error) {
	path, query, _ := strings.Cut(rawPath, "?")
	path, err := c.renderURITemplate(path)
	if err != nil {
		return "", err
	}

	u := base.JoinPath(path)
	queryValues := u.Query()
	if as := c.Account; impersonate && as != "" {
		queryValues.Set("as", as)
	}

	urlQuery := queryValues.Encode()
	if urlQuery != "" && query != "" {
		urlQuery += "&"
	}

	u.RawQuery = urlQuery + query
	return u.String(), nil
}

// endpointPath computes the relative the path (with query) of a URL below the
// API endpoint, with its query. Returns false when the URL is served elsewhere
func (c *Client) endpointPath(rawURL string) (string, bool) {
	base := c.Endpoint
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != base.Scheme || !strings.EqualFold(u.Host, base.Host) {
		return "", false
	}

	path, ok := strings.CutPrefix(u.EscapedPath(), strings.TrimSuffix(base.EscapedPath(), "/"))
	if !ok || !strings.HasPrefix(path, "/") {
		return "", false
	}

	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return path, true
}

// Populate API request body as JSON with the proper Content-Type header
func (c *Client) prepareJSONBody(req *request, data any) error {
	if req.err != nil {
		return req.err
	}

	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(body))
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

// API request to be executed on the client, with its timeout and retries
type request struct {
	*http.Request
	err error

	timeout time.Duration
	onRetry func(string)
	retries int
}

// Common request processing (standard semantics for closing resp.Body)
func (r *request) doCommon() (*http.Response, error) {
	if r.err != nil {
		return nil, r.err
	}

	resp, err := DefaultConduit.Do(r)
	if err != nil {
		return resp, timeoutErr(err)
	}

	if err := DecodeResponseError(resp); err != nil {
		drainAndClose(resp.Body)
		return resp, err
	}

	return resp, nil
}

// Fetch and decode JSON from Gemfury with Authentication, returns error
func (r *request) doJSON(data any) error {
	_, err := r.doPaginatedJSON(data)
	return err
}

// Fetch and decode JSON from Gemfury with Authentication, returns pagination and error
func (r *request) doPaginatedJSON(data any) (*PaginationResponse, error) {
	resp, err := r.doCommon()
	if err != nil {
		r.err = err
		return nil, err
	}

	defer drainAndClose(resp.Body)

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

// A connection is reused only if the body of its response was read to the end
func drainAndClose(body io.ReadCloser) {
	io.CopyN(io.Discard, body, maxDrainBytes)
	body.Close()
}

// Create Gemfury API request, and then stream output
func (r *request) doWithOutput(out io.Writer) error {
	resp, err := r.doCommon()
	if err != nil {
		r.err = err
		return err
	}

	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	r.err = timeoutErr(err)
	return r.err
}

// timeoutErr is ErrTimeout for a request, or the read of its response,
// that timed out, or else err as it is
func timeoutErr(err error) error {
	if ne := net.Error(nil); errors.As(err, &ne) && ne.Timeout() {
		return ErrTimeout
	}
	return err
}

// IsConnectionError reports whether err is a failure to reach the server,
// or to receive all of its response. It is told by its type, as every
// failed request is a net.Error, by its url.Error. A connection closed by
// the server is an EOF for the request, and an unexpected one for the body.
func IsConnectionError(err error) bool {
	var opErr *net.OpError
	var dnsErr *net.DNSError
	var urlErr *url.Error

	return errors.As(err, &opErr) || errors.As(err, &dnsErr) ||
		errors.As(err, &urlErr) && errors.Is(urlErr, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}

// timeoutBody is a response body whose read fails as ErrTimeout, rather
// than as a bare net.Error, when the timeout of its request runs out
type timeoutBody struct {
	io.ReadCloser
}

func (b timeoutBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	return n, timeoutErr(err)
}

// conduitStandard is the wrapper for net/http that builds each request, and
// sends it with its timeout and retries. See DefaultConduit, the only one.
type conduitStandard struct {
	Transport http.RoundTripper
	Version   string
}

// NewRequest builds a request of the API, with the headers that every one has
func (c *conduitStandard) NewRequest(cc context.Context, method, rawURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(cc, method, rawURL, body)
	if err != nil {
		return req, err
	}

	req.Header.Set("User-Agent", fmt.Sprintf(hdrUserAgent, c.Version))
	req.Header.Add("Accept", hdrAcceptAPIv1)
	return req, nil
}

// Do sends the request, within its timeout. Only a request that reads, by
// GET or HEAD, is sent again on a failure: an upload that was stored would
// be refused as a duplicate, and its body cannot be replayed.
func (c *conduitStandard) Do(r *request) (*http.Response, error) {
	hc := &http.Client{Transport: c.Transport, Timeout: r.timeout}
	if m := r.Method; (m != http.MethodGet && m != http.MethodHead) || r.retries == 0 {
		return hc.Do(r.Request)
	}

	rreq, err := retryablehttp.FromRequest(r.Request)
	if err != nil {
		return nil, err
	}

	rc := &retryablehttp.Client{
		HTTPClient:   hc,
		RetryMax:     r.retries,
		RetryWaitMin: retryWaitMin,
		RetryWaitMax: retryWaitMax,
		CheckRetry:   retryPolicy,
		Backoff:      r.backoff,
		// The last response is handed back as it is, so that its error is
		// reported as any other, with its request ID
		ErrorHandler: retryablehttp.PassthroughErrorHandler,
	}

	return rc.Do(rreq)
}

// retryPolicy tells whether a failed request is sent again: one that was
// rate limited, that the server could not serve for now, or that failed
// to connect or lost its connection. Not one that was interrupted, nor
// one that timed out, which would only time out again, nor one refused
// for good, such as by a certificate.
func retryPolicy(cc context.Context, resp *http.Response, err error) (bool, error) {
	if cc.Err() != nil {
		return false, nil
	}

	if err != nil {
		return !errors.Is(timeoutErr(err), ErrTimeout) && IsConnectionError(err), nil
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true, nil
	}
	return false, nil
}

// backoff is how long to wait before a request is sent again: as long as
// the server asks, up to retryAfterMax, or else twice the last wait, from
// waitMin up to waitMax. Each wait is told to onRetry first.
func (r *request) backoff(waitMin, waitMax time.Duration, attempt int, resp *http.Response) time.Duration {
	wait := min(waitMin<<min(attempt, 10), waitMax) // Clamped before the shift could overflow, long past the cap
	if after, ok := retryAfter(resp); ok {
		wait = after
	}

	if r.onRetry != nil {
		r.onRetry(retryNotice(resp, wait))
	}
	return wait
}

// retryAfter is the wait that the Retry-After header of a response asks
// for, in seconds or until a date, and whether it asks for one: not when
// there is no response, or the header is absent or malformed. A longer wait
// is retryAfterMax; the seconds are capped before they are scaled, which
// could overflow.
func retryAfter(resp *http.Response) (time.Duration, bool) {
	if resp == nil {
		return 0, false
	}

	header := resp.Header.Get("Retry-After")
	if seconds, err := strconv.Atoi(header); err == nil {
		return min(max(time.Duration(seconds), 0), retryAfterMax/time.Second) * time.Second, true
	} else if at, err := http.ParseTime(header); err == nil {
		return min(max(time.Until(at), 0), retryAfterMax), true
	}
	return 0, false
}

// retryNotice says why a request is sent again, and after how long
func retryNotice(resp *http.Response, wait time.Duration) string {
	in := wait.Round(time.Second)
	switch {
	case resp == nil:
		return fmt.Sprintf("Connection failed. Retrying in %s", in)
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Sprintf("Rate limited. Retrying in %s", in)
	}
	return fmt.Sprintf("Unavailable (HTTP %d). Retrying in %s", resp.StatusCode, in)
}
