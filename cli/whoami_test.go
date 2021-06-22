package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"context"
	"strings"
	"testing"
)

const whoamiResponse = `{
	"name": "joetest"
}`

func TestWhoamiCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServer("GET", "/users/me", whoamiResponse, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := cli.ContextGlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommand(cc, []string{"whoami"})
	if err != nil {
		t.Fatalf("Command error: %s", err)
	}

	errStr := string(term.ErrBytes())
	if errStr != "" {
		t.Errorf("Error output: %q", errStr)
	}

	outStr := string(term.OutBytes())
	if exp := "You are logged in as \"joetest\"\n"; outStr != exp {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestWhoamiCommandForbidden(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServer("GET", "/users/me", "{}", 403)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := cli.ContextGlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommand(cc, []string{"whoami"})
	if err != api.ErrForbidden {
		t.Fatalf("Command error: %s", err)
	}

	errStr := string(term.ErrBytes())
	if exp := "Error: You're not allowed to do this\n"; errStr != exp {
		t.Errorf("Error should be %q, got %q", exp, errStr)
	}

	outStr := string(term.OutBytes())
	if !strings.HasPrefix(outStr, "Usage:\n") {
		t.Errorf("Output isn't showing usage: \n%s", outStr)
	}
}

func runCommand(cc context.Context, args []string) error {
	cmd := cli.NewRootCommand(cc)
	cmd.SetArgs([]string{"whoami"})
	return cmd.ExecuteContext(cc)
}
