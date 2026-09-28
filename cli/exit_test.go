package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"testing"
)

// apiError is an error from the body of an API response, by type and status
func apiError(errType string, status int) error {
	return api.UserError{Message: "As the API has it", Type: errType, Status: status}
}

func TestExitStatus(t *testing.T) {
	for name, tc := range map[string]struct {
		err error
		exp int
	}{
		"success":                  {nil, cli.ExitOK},
		"any other error":          {errors.New("Failed"), cli.ExitError},
		"forbidden":                {api.ErrForbidden, cli.ExitError},
		"forbidden, in body":       {apiError("Denied", 403), cli.ExitError},
		"invalid, in body":         {apiError("GemVersionError", 422), cli.ExitError},
		"status unknown, in body":  {apiError("Unknown", 0), cli.ExitError},
		"no one to ask":            {terminal.ErrNoInput, cli.ExitUsage},
		"usage, of the API":        {cli.AsUsageError(apiError("NotFound", 404)), cli.ExitUsage},
		"not found":                {api.ErrNotFound, cli.ExitNotFound},
		"not found, in body":       {apiError("NotFound", 404), cli.ExitNotFound},
		"not found, wrapped":       {fmt.Errorf("Problem: %w", api.ErrNotFound), cli.ExitNotFound},
		"token refused":            {api.ErrUnauthorized, cli.ExitAuth},
		"token refused, in body":   {apiError("Expired", 401), cli.ExitAuth},
		"version exists":           {api.ErrAlreadyExists, cli.ExitExists},
		"version exists, in body":  {apiError("DupeVersion", 422), cli.ExitExists},
		"not implemented":          {api.ErrNotImplemented, cli.ExitError},
		"not implemented, in body": {apiError("Unsupported", 501), cli.ExitError},
		"too many requests":        {api.ErrTooManyRequests, cli.ExitUnavailable},
		"too many, in body":        {apiError("RateLimit", 429), cli.ExitUnavailable},
		"locked":                   {api.ErrConflict, cli.ExitUnavailable},
		"locked, in body":          {apiError("Locked", 409), cli.ExitUnavailable},
		"timeout":                  {api.ErrTimeout, cli.ExitUnavailable},
		"timeout, in body":         {apiError("Timeout", 408), cli.ExitUnavailable},
		"server error":             {api.ErrFuryServer, cli.ExitUnavailable},
		"server error, in body":    {apiError("Internal", 503), cli.ExitUnavailable},
		"connection closed":        {&url.Error{Op: "Get", URL: "/", Err: io.EOF}, cli.ExitUnavailable},
		"response cut short":       {fmt.Errorf("Decoding: %w", io.ErrUnexpectedEOF), cli.ExitUnavailable},
		"end of other input":       {io.EOF, cli.ExitError},
	} {
		t.Run(name, func(t *testing.T) {
			if got := cli.ExitStatus(tc.err); got != tc.exp {
				t.Errorf("Expected exit status %d, got %d", tc.exp, got)
			}
		})
	}
}

// The help of the root command lists each of the exit statuses
func TestExitStatusHelp(t *testing.T) {
	term := terminal.NewForTest()
	cc := testContext(t, term, terminal.TestAuther("", "", nil), offlineServer(t))
	if err := runCommandNoErr(cc, []string{"--help"}); err != nil {
		t.Fatal(err)
	}

	out := term.OutBytes()
	for _, status := range []int{
		cli.ExitOK, cli.ExitError, cli.ExitUsage, cli.ExitNotFound,
		cli.ExitAuth, cli.ExitExists, cli.ExitUnavailable, 130, 143,
	} {
		listed := regexp.MustCompile(fmt.Sprintf(`(?m)^  .*\b%d\b`, status))
		if !listed.Match(out) {
			t.Errorf("Expected exit status %d in help, got %q", status, out)
		}
	}
}

// An error of the API is of a kind by its type, whatever its status.
// Without such a type it is by its status, as TestExitStatus has it.
func TestUserErrorIs(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		kind error
	}{
		"unauthorized":   {apiError("Unauthorized", 404), api.ErrUnauthorized},
		"forbidden":      {apiError("Forbidden", 404), api.ErrForbidden},
		"version exists": {apiError("DupeVersion", 409), api.ErrAlreadyExists},
		"conflict":       {apiError("Conflict", 409), api.ErrAlreadyExists},
	} {
		t.Run(name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.kind) {
				t.Errorf("Expected an error that is %q", tc.kind)
			}

			for _, other := range []error{
				api.ErrUnauthorized, api.ErrForbidden, api.ErrNotFound,
				api.ErrAlreadyExists, api.ErrConflict,
			} {
				if other != tc.kind && errors.Is(tc.err, other) {
					t.Errorf("Expected an error that is not %q", other)
				}
			}
		})
	}
}

// The exit status of a command with several items to fail on
func TestCommandExitStatusSeveralItems(t *testing.T) {
	// Where yank looks up its versions, before it removes any
	lookup := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("name") {
		case "missing":
			respondWith(404, "")(w, r)
		case "secret":
			respondWith(403, "")(w, r)
		default:
			respondWith(200, versionsResponses[0])(w, r)
		}
	}

	for name, tc := range map[string]struct {
		args []string
		exp  int
	}{
		"each item not found":  {[]string{"missing@1.0", "missing@2.0"}, cli.ExitNotFound},
		"one item not found":   {[]string{"foo@1.0", "missing@1.0"}, cli.ExitNotFound},
		"items fail otherwise": {[]string{"missing@1.0", "secret@1.0"}, cli.ExitError},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("GET /versions", lookup)
			})

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, append([]string{"yank", "--force"}, tc.args...))
			expectExitStatus(t, term, err, tc.exp)
		})
	}
}

// An error says what the server said of it, in any shape of its errors,
// or else what its type or status stands for. It is about what the
// command asked for, unless it is about the credentials or the server, and
// that of a failing server has what to report it by.
func TestCommandErrorMessage(t *testing.T) {
	const about = "Package \"foo\": "
	const notFound = about + "Doesn't look like this exists"
	const failed = "Something went wrong. Please contact support."

	for name, tc := range map[string]struct {
		status    int
		requestID string
		body      string
		message   string
		exp       int
	}{
		"a string":             {400, "req-1", `{"error": "Name is taken"}`, about + "Name is taken", cli.ExitError},
		"an object":            {404, "req-1", `{"error": {"type": "NotFound", "message": "No such package"}}`, about + "No such package", cli.ExitNotFound},
		"an object, of a type": {403, "req-1", `{"error": {"type": "Forbidden", "message": "Token is read-only"}}`, about + "Token is read-only", cli.ExitError},
		"a list, its first":    {404, "req-1", ` [{}, {"error": "No such package"}, {"error": "Nor this"}, 404]`, about + "No such package", cli.ExitNotFound},
		"a list of no errors":  {404, "req-1", `[{}, 404]`, notFound, cli.ExitNotFound},
		"a type of no shape":   {404, "req-1", `{"error": {"message": "No such package", "type": 404}}`, about + "No such package", cli.ExitNotFound},
		"no message":           {404, "req-1", `{"error": {"type": "NotFound"}}`, notFound, cli.ExitNotFound},
		"no message, a type":   {404, "req-1", `{"error": {"type": "Forbidden"}}`, about + "You're not allowed to do this", cli.ExitError},
		"no error":             {404, "req-1", `{"error": null}`, notFound, cli.ExitNotFound},
		"an error of no shape": {404, "req-1", `{"error": 404}`, notFound, cli.ExitNotFound},
		"not JSON":             {404, "req-1", `<html>Not Found</html>`, notFound, cli.ExitNotFound},
		"not a success":        {304, "req-1", ``, about + "Not Modified", cli.ExitError},
		"not implemented":      {501, "req-1", ``, about + "This operation is not supported", cli.ExitError},
		"credentials":          {401, "req-1", `{"error": "Token has expired"}`, "Token has expired", cli.ExitAuth},
		"too many requests":    {429, "req-1", ``, "Too many requests. Try again later.", cli.ExitUnavailable},
		"timeout":              {408, "req-1", ``, "Operation timed out. Try again later.", cli.ExitUnavailable},
		"server, to report":    {502, "req-1", `<html>Bad Gateway</html>`, failed + " (HTTP 502, request ID req-1)", cli.ExitUnavailable},
		"server, with no ID":   {502, "", ``, failed + " (HTTP 502)", cli.ExitUnavailable},
		"server, in its words": {500, "req-1", `{"error": "Out of disk"}`, "Out of disk (HTTP 500, request ID req-1)", cli.ExitUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/packages/foo/versions", func(w http.ResponseWriter, r *http.Request) {
					if tc.requestID != "" {
						w.Header().Set("X-Request-Id", tc.requestID)
					}
					respondWith(tc.status, tc.body)(w, r)
				})
			})

			cc := testContext(t, term, auth, server)
			expectExitStatus(t, term, runCommand(cc, []string{"versions", "foo"}), tc.exp)
			expectOutput(t, term, "", "Error: "+tc.message+"\n")
		})
	}
}

// Without credentials: for lack of a terminal to login
// from, or by backing out of the login at one
func TestCommandExitStatusNotLoggedIn(t *testing.T) {
	for name, tc := range map[string]struct {
		args []string
		user bool // At the terminal
	}{
		"not logged in":    {[]string{"packages"}, false},
		"login unattended": {[]string{"login"}, false},
		"login cancelled":  {[]string{"packages"}, true},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("", "", nil)
			term := terminal.NewForTest()
			term.SetInteractive(tc.user)
			term.InWrite([]byte("q")) // Backs out of the login, if asked

			// Only the default handlers of login, which is never completed
			server := testutil.APIServerCustom(t, func(*http.ServeMux) {})

			cc := testContext(t, term, auth, server)
			expectExitStatus(t, term, runCommand(cc, tc.args), cli.ExitAuth)
			expectCredentials(t, auth, "", "")
		})
	}
}

// A server that is down is worth trying again later, unlike an endpoint
// that is not a URL. Neither is about what the command asked for.
func TestCommandExitStatusUnreachable(t *testing.T) {
	server := offlineServer(t)
	server.Close()

	for name, tc := range map[string]struct {
		endpoint string
		exp      int
	}{
		"server is down":  {server.URL, cli.ExitUnavailable},
		"not an endpoint": {"://invalid", cli.ExitError},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			cc := testContext(t, term, auth, server)
			ctx.GlobalFlags(cc).Endpoint = tc.endpoint
			expectExitStatus(t, term, runCommand(cc, []string{"versions", "foo"}), tc.exp)
			if stderr := term.ErrBytes(); len(stderr) == 0 || bytes.Contains(stderr, []byte("Package ")) {
				t.Errorf("Expected an error that is not about the package, got %q", stderr)
			}
		})
	}
}
