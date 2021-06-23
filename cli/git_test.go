package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"
	"strings"
	"testing"
)

const (
	gitRebuildResponse = "Build done!\n"
)

// ==== GIT REBUILD ====

func TestGitRebuildCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/git/repos/me/repo-name/builds"
	server := testutil.APIServer(t, "POST", path, gitRebuildResponse, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := cli.ContextGlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommand(cc, []string{"git", "rebuild", "repo-name"})
	if err != nil {
		t.Fatalf("Command error: %s", err)
	}

	errStr := string(term.ErrBytes())
	if errStr != "" {
		t.Errorf("Error output: %q", errStr)
	}

	outStr := string(term.OutBytes())
	if exp := gitRebuildResponse; !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestGitRebuildCommandUnauthorized(t *testing.T) {
	path := "/git/repos/me/repo-name/builds"
	server := testutil.APIServer(t, "POST", path, gitRebuildResponse, 200)
	testCommandLoginPreCheck(t, []string{"git", "rebuild", "repo-name"}, server)
	server.Close()
}

func TestGitRebuildCommandForbidden(t *testing.T) {
	path := "/git/repos/me/repo-name/builds"
	server := testutil.APIServer(t, "POST", path, "", 403)
	testCommandForbiddenResponse(t, []string{"git", "rebuild", "repo-name"}, server)
	server.Close()
}

// ==== GIT RENAME ====

func TestGitRenameCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "PATCH", path, "{}", 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := cli.ContextGlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommand(cc, []string{"git", "rename", "repo-name", "new-name"})
	if err != nil {
		t.Fatalf("Command error: %s", err)
	}

	errStr := string(term.ErrBytes())
	if errStr != "" {
		t.Errorf("Error output: %q", errStr)
	}

	exp := "Renamed repo-name repository to new-name\n"
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestGitRenameCommandUnauthorized(t *testing.T) {
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "PATCH", path, "{}", 200)
	args := []string{"git", "rename", "repo-name", "new-name"}
	testCommandLoginPreCheck(t, args, server)
	server.Close()
}

func TestGitRenameCommandForbidden(t *testing.T) {
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "PATCH", path, "", 403)
	args := []string{"git", "rename", "repo-name", "new-name"}
	testCommandForbiddenResponse(t, args, server)
	server.Close()
}

// ==== GIT RESET ====

func TestGitResetCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "DELETE", path, "{}", 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := cli.ContextGlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommand(cc, []string{"git", "reset", "repo-name"})
	if err != nil {
		t.Fatalf("Command error: %s", err)
	}

	errStr := string(term.ErrBytes())
	if errStr != "" {
		t.Errorf("Error output: %q", errStr)
	}

	outStr := string(term.OutBytes())
	if exp := "Removed repo-name repository\n"; !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestGitResetCommandUnauthorized(t *testing.T) {
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "DELETE", path, "{}", 200)
	testCommandLoginPreCheck(t, []string{"git", "reset", "repo-name"}, server)
	server.Close()
}

func TestGitResetCommandForbidden(t *testing.T) {
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "DELETE", path, "", 403)
	testCommandForbiddenResponse(t, []string{"git", "reset", "repo-name"}, server)
	server.Close()
}
