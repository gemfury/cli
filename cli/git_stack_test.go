package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	gitInfoResponse = `{ "repo": {
		"build_stack": { "name": "fury-14" },
    "name": "repo-name"
  }}
`
	gitStacksResponse = `[
	  {"name":"fury-14"},
	  {"name":"fury-22"}
	]
`
)

// ==== GIT STACK ====

func TestGitStackCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server with both repo info and stack listing
	server := testGitStackServer(t)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"git", "stack", "repo-name"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "* fury-14 fury-22"
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestGitStackCommandUnauthorized(t *testing.T) {
	server := testGitStackServer(t)
	testCommandLoginPreCheck(t, []string{"git", "stack", "repo-name"}, server)
	server.Close()
}

func TestGitStackCommandForbidden(t *testing.T) {
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "GET", path, "", 403)
	testCommandForbiddenResponse(t, []string{"git", "stack", "repo-name"}, server)
	server.Close()
}

func testGitStackServer(t *testing.T) *httptest.Server {
	return testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/git/repos/me/repo-name", func(w http.ResponseWriter, r *http.Request) {
			t.Logf("API Request: %s %s", r.Method, r.URL.String())
			w.Write([]byte(gitInfoResponse))
		})
		mux.HandleFunc("/git/stacks", func(w http.ResponseWriter, r *http.Request) {
			t.Logf("API Request: %s %s", r.Method, r.URL.String())
			w.Write([]byte(gitStacksResponse))
		})
	})
}

// ==== GIT STACK SET ====

func TestGitStackSetCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "PATCH", path, "{}", 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"git", "stack", "set", "repo-name", "fury-22"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "Updated repo-name repository build stack"
	if outStr := compactString(term.OutBytes()); outStr != exp {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestGitStackSetCommandUnauthorized(t *testing.T) {
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "PATCH", path, "{}", 200)
	testCommandLoginPreCheck(t, []string{"git", "stack", "set", "repo-name", "fury-22"}, server)
	server.Close()
}

func TestGitStackSetCommandForbidden(t *testing.T) {
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "PATCH", path, "", 403)
	testCommandForbiddenResponse(t, []string{"git", "stack", "set", "repo-name", "fury-22"}, server)
	server.Close()
}
