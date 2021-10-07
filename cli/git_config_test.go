package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"
	"strings"
	"testing"
)

const (
	gitConfigResponse = `{ "config_vars": {
    "KEY1": "VALUE1",
    "KEY2": "VALUE2"
  }}
`
)

// ==== GIT CONFIG ====

func TestGitConfigCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/git/repos/me/repo-name/config-vars"
	server := testutil.APIServer(t, "GET", path, gitConfigResponse, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"git", "config", "repo-name"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "KEY1: VALUE1 KEY2: VALUE2"
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestGitConfigCommandUnauthorized(t *testing.T) {
	path := "/git/repos/me/repo-name/config-vars"
	server := testutil.APIServer(t, "GET", path, "{}", 200)
	testCommandLoginPreCheck(t, []string{"git", "config", "repo-name"}, server)
	server.Close()
}

func TestGitConfigCommandForbidden(t *testing.T) {
	path := "/git/repos/me/repo-name/config-vars"
	server := testutil.APIServer(t, "GET", path, "", 403)
	testCommandForbiddenResponse(t, []string{"git", "config", "repo-name"}, server)
	server.Close()
}

// ==== GIT CONFIG GET ====

func TestGitConfigGetCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/git/repos/me/repo-name/config-vars"
	server := testutil.APIServer(t, "GET", path, gitConfigResponse, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"git", "config", "get", "repo-name", "KEY2"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "KEY2: VALUE2"
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	} else if strings.Contains(outStr, "KEY1") {
		t.Errorf("Expected output to be filtered, got %q", outStr)
	}
}

func TestGitConfigGetCommandUnauthorized(t *testing.T) {
	path := "/git/repos/me/repo-name/config-vars"
	server := testutil.APIServer(t, "GET", path, "{}", 200)
	testCommandLoginPreCheck(t, []string{"git", "config", "get", "repo-name", "KEY2"}, server)
	server.Close()
}

func TestGitConfigGetCommandForbidden(t *testing.T) {
	path := "/git/repos/me/repo-name/config-vars"
	server := testutil.APIServer(t, "GET", path, "", 403)
	testCommandForbiddenResponse(t, []string{"git", "config", "get", "repo-name", "KEY2"}, server)
	server.Close()
}

// ==== GIT CONFIG SET ====

func TestGitConfigSetCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/git/repos/me/repo-name/config-vars"
	server := testutil.APIServer(t, "PATCH", path, gitConfigResponse, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"git", "config", "set", "repo-name", "KEY2=VALUE2"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "Updated repo-name repository config"
	if outStr := compactString(term.OutBytes()); outStr != exp {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestGitConfigSetCommandUnauthorized(t *testing.T) {
	path := "/git/repos/me/repo-name/config-vars"
	server := testutil.APIServer(t, "PATCH", path, "{}", 200)
	testCommandLoginPreCheck(t, []string{"git", "config", "set", "repo-name", "KEY2=VALUE2"}, server)
	server.Close()
}

func TestGitConfigSetCommandForbidden(t *testing.T) {
	path := "/git/repos/me/repo-name/config-vars"
	server := testutil.APIServer(t, "PATCH", path, "", 403)
	testCommandForbiddenResponse(t, []string{"git", "config", "set", "repo-name", "KEY2=VALUE2"}, server)
	server.Close()
}
