package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

var (
	usageRegexp = regexp.MustCompilePOSIX("^Usage:$")
)

// Top-level testing initializer
func TestMain(m *testing.M) {
	os.Setenv("TZ", "US/Pacific")
	os.Exit(m.Run())
}

func TestRootCommand(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Fire up test server (error on everything)
	server := testutil.APIServer(t, "", "/", "", 501)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	if err := runCommandNoErr(cc, []string{""}); err != nil {
		t.Fatal(err)
	}

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

func runCommandNoErr(cc context.Context, args []string) error {
	if err := runCommand(cc, args); err != nil {
		return fmt.Errorf("Command error: %w", err)
	}

	term := ctx.TestTerm(cc)
	if errStr := string(term.ErrBytes()); errStr != "" {
		return fmt.Errorf("Error output: %q", errStr)
	}

	return nil
}

// We first test with manual (prompt) login, and then test with "--api-token" flag
func testCommandLoginPreCheck(t *testing.T, args []string, server *httptest.Server) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.PushEndpoint = server.URL
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

	// Testing with "--api-token" should skip calling Auth() on TestAuther
	auth = terminal.TestAuther("", "", fmt.Errorf("TestAuther should not be called"))
	cc = cli.TestContext(term, auth)
	flags = ctx.GlobalFlags(cc)
	flags.PushEndpoint = server.URL
	flags.Endpoint = server.URL

	args = append(args, "--api-token", "abc123")
	if err := runCommand(cc, args); err != nil {
		t.Errorf("Command error: %s", err)
	}
}

func testCommandForbiddenResponse(t *testing.T, args []string, server *httptest.Server) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.PushEndpoint = server.URL
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
