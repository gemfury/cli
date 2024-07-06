package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"strings"
	"testing"
)

// Login is more or less the same as the "whoami" command
// because all commands force a login if logged out
// /login route is already present on APIServer

func TestLoginCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServer(t, "GET", "/users/me", whoamiResponse, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	// Add any key for the "open browser" prompt
	term.InWrite([]byte("!"))

	err := runCommandNoErr(cc, []string{"login"})
	if err != nil {
		t.Error(err)
	}

	outStr := string(term.OutBytes())
	if exp := "You are logged in as \"u@example.com\"\n"; !strings.Contains(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestLoginCommandInteractive(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServer(t, "GET", "/users/me", whoamiResponse, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	term.SetPromptResponses(map[string]string{
		"Email: ":    "u@example.com",
		"Password: ": "secreto",
	})

	err := runCommandNoErr(cc, []string{"login", "--interactive"})
	if err != nil {
		t.Error(err)
	}

	outStr := string(term.OutBytes())
	if exp := "You are logged in as \"u@example.com\"\n"; !strings.Contains(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestLoginCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/users/me", whoamiResponse, 200)
	testCommandLoginPreCheck(t, []string{"login"}, server, noLoginOpt)
	server.Close()
}

// Login is more or less the same as the "whoami" command
// because all commands force a login if logged out

func TestLogoutCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServer(t, "POST", "/logout", "", 200)
	defer server.Close()

	term.SetPromptResponses(map[string]string{
		"Are you sure you want to logout? [y/N]": "Y",
	})

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"logout"})
	if err != nil {
		t.Error(err)
	}

	outStr := string(term.OutBytes())
	if exp := "You have been logged out\n"; outStr != exp {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	if auth.User != "" || auth.Pass != "" || auth.Err != nil {
		t.Errorf("Expected command to wipe auth: %+v", auth)
	}
}

func TestLogoutCommandAbort(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server (should not be called)
	server := testutil.APIServer(t, "GET", "/", "", 200)
	defer server.Close()

	term.SetPromptResponses(map[string]string{
		"Are you sure you want to logout? [y/N]": "ABORT",
	})

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"logout"})
	if err != nil {
		t.Error(err)
	}

	outStr := string(term.OutBytes())
	if exp := ""; outStr != exp {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	if auth.User != "user" || auth.Pass != "abc123" || auth.Err != nil {
		t.Errorf("Expected command to retain auth: %+v", auth)
	}
}
