package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"context"
	"errors"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

var (
	usageRegexp = regexp.MustCompilePOSIX("^Usage:$")
)

func TestRootCommand(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Fire up test server (error on everything)
	server := testutil.APIServer(t, "", "/", "", 501)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := cli.ContextGlobalFlags(cc)
	flags.Endpoint = server.URL

	runCommand(cc, []string{""})

	outStr := string(term.OutBytes())
	if exp := "See https://gemfury.com/help/gemfury-cli\n"; !strings.HasPrefix(outStr, exp) {
		t.Errorf("Expected output to start with %q, got %q", exp, outStr)
	}
}

func runCommand(cc context.Context, args []string) error {
	cmd := cli.NewRootCommand(cc)
	cmd.SetArgs(args)
	return cmd.ExecuteContext(cc)
}

func testCommandLoginPreCheck(t *testing.T, args []string, server *httptest.Server) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	cc := cli.TestContext(term, auth)
	flags := cli.ContextGlobalFlags(cc)
	flags.Endpoint = server.URL

	// Prepare for login prompt
	term.SetPromptResponses(map[string]string{
		"Email: ":    "user@example.com",
		"Password: ": "secreto",
	})

	if err := runCommand(cc, args); err != nil {
		t.Errorf("Command error: %s", err)
	}

	if u, p, err := auth.Auth(); err != nil {
		t.Errorf("Login error: %s", err)
	} else if exp := "u@example.com"; u != exp {
		t.Errorf("Expected user %q, got %q", exp, u)
	} else if exp := "token-abc-123"; p != exp {
		t.Errorf("Expected pass %q, got %q", exp, p)
	}
}

func testCommandForbiddenResponse(t *testing.T, args []string, server *httptest.Server) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	cc := cli.TestContext(term, auth)
	flags := cli.ContextGlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommand(cc, args)
	if !errors.Is(err, api.ErrForbidden) {
		t.Fatalf("Command error: %s", err)
	}

	errStr := string(term.ErrBytes())
	if exp := "Error: You're not allowed to do this\n"; errStr != exp {
		t.Errorf("Error should be %q, got %q", exp, errStr)
	}

	if ob := term.OutBytes(); !usageRegexp.Match(ob) {
		t.Errorf("Output isn't showing usage: \n%s", ob)
	}
}
