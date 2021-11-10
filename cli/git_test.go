package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"net/http"
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
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"git", "rebuild", "repo-name"})
	if err != nil {
		t.Fatal(err)
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
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"git", "rename", "repo-name", "new-name"})
	if err != nil {
		t.Fatal(err)
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

// ==== GIT DESTROY ====

func TestGitDestroyCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Destroy without reset
	path := "/git/repos/me/repo-name"
	serverDestroy := testutil.APIServerCustom(t, "DELETE", path, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("reset") {
			t.Errorf("Has extraneous reset=1 URL query")
		}
		w.Write([]byte("{}"))
	})
	defer serverDestroy.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = serverDestroy.URL

	err := runCommandNoErr(cc, []string{"git", "destroy", "repo-name"})
	if err != nil {
		t.Fatal(err)
	}

	outStr := string(term.OutBytes())
	if exp := "Removed repo-name repository\n"; !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	// Reset without destroying repo
	serverReset := testutil.APIServerCustom(t, "DELETE", path, func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query(); q.Get("reset") != "1" {
			t.Errorf("Missing reset=1 URL query")
		}
		w.Write([]byte("{}"))
	})
	defer serverReset.Close()

	// Via "--reset-only" option
	cc = cli.TestContext(term, auth)
	flags = ctx.GlobalFlags(cc)
	flags.Endpoint = serverReset.URL

	err = runCommandNoErr(cc, []string{"git", "destroy", "--reset-only", "repo-name"})
	if err != nil {
		t.Fatal(err)
	}

	outStr = string(term.OutBytes())
	if exp := "Reset repo-name repository\n"; !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	// Via "git:reset" command
	cc = cli.TestContext(term, auth)
	flags = ctx.GlobalFlags(cc)
	flags.Endpoint = serverReset.URL

	err = runCommandNoErr(cc, []string{"git", "reset", "repo-name"})
	if err != nil {
		t.Fatal(err)
	}

	outStr = string(term.OutBytes())
	if exp := "Reset repo-name repository\n"; !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestGitDestroyCommandUnauthorized(t *testing.T) {
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "DELETE", path, "{}", 200)
	testCommandLoginPreCheck(t, []string{"git", "destroy", "repo-name"}, server)
	server.Close()
}

func TestGitDestroyCommandForbidden(t *testing.T) {
	path := "/git/repos/me/repo-name"
	server := testutil.APIServer(t, "DELETE", path, "", 403)
	testCommandForbiddenResponse(t, []string{"git", "destroy", "repo-name"}, server)
	server.Close()
}

// ==== GIT LIST ====

var gitReposResponses = []string{`{ "repos": [{
	"id": "repo_a1b2c3",
	"name": "repoA"
}]}`, `{ "repos" : [{
	"id": "repo_z1y2x3",
	"name": "repoZ"
}]}`}

func TestGitListCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/git/repos/me"
	server := testutil.APIServerPaginated(t, "GET", path, gitReposResponses, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"git", "list"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "*** GEMFURY GIT REPOS *** repoA repoZ"
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestGitListCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/git/repos/me", "{}", 200)
	testCommandLoginPreCheck(t, []string{"git", "list"}, server)
	server.Close()
}

func TestGitListCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/git/repos/me", "", 403)
	testCommandForbiddenResponse(t, []string{"git", "list"}, server)
	server.Close()
}
