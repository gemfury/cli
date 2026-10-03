package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	gitRebuildResponse = "Build done!\n"
)

// ==== GIT REBUILD ====

func TestGitRebuildCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Revision input, output
	revSrc := "refs/tags/v1.2.3"
	var revDst string

	// Fire up test server
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc(gitBuildsPath, func(w http.ResponseWriter, r *http.Request) {
			if m := r.Method; m != "POST" {
				t.Errorf("Incorrect method: %q", m)
			}
			revDst = r.URL.Query().Get("build[revision]")
			w.Write([]byte(gitRebuildResponse))
		})
	})

	cc := cli.TestContext(t.Context(), term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	// No revision specified
	err := runCommandNoErr(cc, []string{"git", "rebuild", "repo-name"})
	if err != nil {
		t.Fatal(err)
	}

	outStr := string(term.OutBytes())
	if exp := gitRebuildResponse; !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	} else if revDst != "" {
		t.Errorf("Expected no revision, got %q", revDst)
	}

	// Revision specified with full flag
	err = runCommandNoErr(cc, []string{"git", "rebuild", "repo-name", "--revision", revSrc})
	if err != nil {
		t.Fatal(err)
	}

	outStr = string(term.OutBytes())
	if exp := gitRebuildResponse; !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	} else if revDst != revSrc {
		t.Errorf("Expected %q revision, got %q", revSrc, revDst)
	}

	// Revision specified with short flag
	err = runCommandNoErr(cc, []string{"git", "rebuild", "repo-name", "-r", revSrc})
	if err != nil {
		t.Fatal(err)
	} else if revDst != revSrc {
		t.Errorf("Expected %q revision, got %q", revSrc, revDst)
	}

	// Revision specified with "@" separator
	err = runCommandNoErr(cc, []string{"git", "rebuild", "repo-name@" + revSrc})
	if err != nil {
		t.Fatal(err)
	} else if revDst != revSrc {
		t.Errorf("Expected %q revision, got %q", revSrc, revDst)
	}
}

func TestGitRebuildCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "POST", gitBuildsPath, gitRebuildResponse, 200)
	testCommandLoginPreCheck(t, []string{"git", "rebuild", "repo-name"}, server)
}

// The error is about the revision of the repository, when one is given
func TestGitRebuildCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "POST", gitBuildsPath, "", 403)

	for about, args := range map[string][]string{
		`Repository "repo-name"`:      {"repo-name"},
		`Repository "repo-name@v1.0"`: {"repo-name@v1.0"},
		`Repository "repo-name@v2.0"`: {"repo-name", "--revision", "v2.0"},
	} {
		t.Run(about, func(t *testing.T) {
			testCommandForbiddenResponse(t, append([]string{"git", "rebuild"}, args...), server, about)
		})
	}
}

// ==== GIT RENAME ====

func TestGitRenameCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServer(t, "PATCH", gitRepoPath, "{}", 200)

	cc := cli.TestContext(t.Context(), term, auth)
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
	server := testutil.APIServer(t, "PATCH", gitRepoPath, "{}", 200)
	args := []string{"git", "rename", "repo-name", "new-name"}
	testCommandLoginPreCheck(t, args, server)
}

func TestGitRenameCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "PATCH", gitRepoPath, "", 403)
	args := []string{"git", "rename", "repo-name", "new-name"}
	testCommandForbiddenResponse(t, args, server, `Repository "repo-name"`)
}

// ==== GIT DESTROY ====

const (
	confirmRemove = "Are you sure you want to remove the repo-name repository? [y/N]"
	confirmReset  = "Are you sure you want to reset the repo-name repository? [y/N]"
)

// A repository is removed, or reset, once that is confirmed or forced
func TestGitDestroyCommandSuccess(t *testing.T) {
	for name, tc := range map[string]struct {
		args    []string
		confirm string // Not asked when empty
		reset   string // Expected in the URL query
		stdout  string
	}{
		"destroy":         {[]string{"git", "destroy", "repo-name"}, confirmRemove, "", "Removed repo-name repository\n"},
		"destroy --force": {[]string{"git", "destroy", "--force", "repo-name"}, "", "", "Removed repo-name repository\n"},
		"--reset-only":    {[]string{"git", "destroy", "--reset-only", "repo-name"}, confirmReset, "1", "Reset repo-name repository\n"},
		"reset":           {[]string{"git", "reset", "repo-name"}, confirmReset, "1", "Reset repo-name repository\n"},
		"reset -f":        {[]string{"git", "reset", "-f", "repo-name"}, "", "1", "Reset repo-name repository\n"},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			var requests atomic.Int32
			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("DELETE /git/repos/me/repo-name", func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if reset := r.URL.Query().Get("reset"); reset != tc.reset {
						t.Errorf("Expected reset=%q in URL query, got %q", tc.reset, reset)
					}
					w.Write([]byte("{}"))
				})
			})

			// Any other question, or one that is not to be asked, fails the command
			if tc.confirm != "" {
				term.SetPromptResponses(map[string]string{tc.confirm: "Y"})
			}

			cc := testContext(t, term, auth, server)
			if err := runCommandNoErr(cc, tc.args); err != nil {
				t.Fatal(err)
			}

			expectOutput(t, term, tc.stdout, "")
			if n := requests.Load(); n != 1 {
				t.Errorf("Expected 1 request, got %d", n)
			}
		})
	}
}

// Nothing is removed, or reset, unless confirmed
func TestGitDestroyCommandUnconfirmed(t *testing.T) {
	for confirm, args := range map[string][]string{
		confirmRemove: {"git", "destroy", "repo-name"},
		confirmReset:  {"git", "reset", "repo-name"},
	} {
		for answer, interrupted := range unconfirmed {
			t.Run(args[1]+" "+answer, func(t *testing.T) {
				auth := terminal.TestAuther("user", "abc123", nil)
				term := terminal.NewForTest()

				server := offlineServer(t)

				term.SetPromptResponses(map[string]string{confirm: answer})

				cc := testContext(t, term, auth, server)
				expectUnconfirmed(t, term, runCommand(cc, args), interrupted)
				if out := string(term.OutBytes()); out != "" {
					t.Errorf("Expected no output, got %q", out)
				}
			})
		}
	}
}

func TestGitDestroyCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "DELETE", gitRepoPath, "{}", 200)
	testCommandLoginPreCheck(t, []string{"git", "destroy", "--force", "repo-name"}, server)
}

func TestGitDestroyCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "DELETE", gitRepoPath, "", 403)
	testCommandForbiddenResponse(t, []string{"git", "destroy", "--force", "repo-name"}, server, `Repository "repo-name"`)
}

// ==== GIT LOCK ====

// A locked repository is refused with a 409 and, in the body, an error that
// is a string of no type. That is to try again later, unlike the conflict
// of an existing version. The message is that of the server.
func TestGitCommandsLocked(t *testing.T) {
	for _, args := range [][]string{
		{"git", "destroy", "repo-name", "--force"},
		{"git", "rename", "repo-name", "new-name"},
		{"git", "rebuild", "repo-name"},
		{"git", "config", "set", "repo-name", "KEY=value"},
		{"git", "config", "unset", "repo-name", "KEY"},
		{"git", "stack", "set", "repo-name", "stack-name"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/", respondWith(409, `{"error":"Locked by another request"}`))
			})

			cc := testContext(t, term, auth, server)
			expectExitStatus(t, term, runCommand(cc, args), cli.ExitUnavailable)
			expectErrOutput(t, term, "Error: Repository \"repo-name\": Locked by another request\n")
		})
	}
}

// ==== GIT LIST ====

var gitReposResponses = []string{`{ "repos": [{
	"id": "repo_a1b2c3",
	"name": "repoA",
	"build_stack": { "name": "fury-14" }
}]}`, `{ "repos" : [{
	"id": "repo_z1y2x3",
	"name": "repoZ",
	"build_stack": { "name": "fury-22" }
}]}`}

func TestGitListCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/git/repos/me"
	server := testutil.APIServerPaginated(t, "GET", path, gitReposResponses, 200)

	cc := cli.TestContext(t.Context(), term, auth)
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
}

func TestGitListCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/git/repos/me", "", 403)
	testCommandForbiddenResponse(t, []string{"git", "list"}, server, "")
}
