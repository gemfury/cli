package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"errors"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// An upload has a progress bar, and a listing of several pages
// a spinner, unless --no-progress or --quiet leaves them out
func TestNoProgress(t *testing.T) {
	for name, tc := range map[string]struct {
		args   []string
		server func(*testing.T) *httptest.Server
	}{
		"push":     {[]string{"push", samplePackagePath()}, serving("POST", "/uploads", 200, pushResponse)},
		"packages": {[]string{"packages"}, serving("GET", "/packages", 200, packagesResponses...)},
	} {
		for flag, asked := range map[string]int{"": 1, "--no-progress": 0, "--quiet": 0} {
			t.Run(name+" "+flag, func(t *testing.T) {
				auth := terminal.TestAuther("user", "abc123", nil)
				term := terminal.NewForTest()

				cc := testContext(t, term, auth, tc.server(t))
				if err := runCommand(cc, append(tc.args, strings.Fields(flag)...)); err != nil {
					t.Fatal(err)
				}

				if bars, spinners := term.ProgressShown(); bars+spinners != asked {
					t.Errorf("Expected progress to be asked for %d times, got %d", asked, bars+spinners)
				}
			})
		}
	}
}

// With --quiet, a listing is as without, but for its banner
func TestQuietListing(t *testing.T) {
	for name, banner := range map[string]string{
		"packages":   "\n*** GEMFURY PACKAGES ***\n\n",
		"versions":   "\n*** foo versions ***\n\n",
		"sharing":    "*** Collaborators ***\n",
		"accounts":   "",
		"git list":   "\n*** GEMFURY GIT REPOS ***\n\n",
		"git config": "\n*** GIT CONFIG ***\n\n",
		"git stack":  "*** [repo-name] GIT BUILD STACKS ***\n",
	} {
		t.Run(name, func(t *testing.T) {
			tc := jsonCases[name]
			server := tc.server(t)
			output := func(flags ...string) string {
				auth := terminal.TestAuther("user", "abc123", nil)
				term := terminal.NewForTest()

				cc := testContext(t, term, auth, server)
				if err := runCommandNoErr(cc, slices.Concat(flags, strings.Fields(name), tc.args)); err != nil {
					t.Fatal(err)
				}
				return string(term.OutBytes())
			}

			listing, ok := strings.CutPrefix(output(), banner)
			if !ok || listing == "" {
				t.Fatalf("Expected a listing after the banner %q, got %q", banner, listing)
			}

			if out := output("--quiet"); out != listing {
				t.Errorf("Output with --quiet should be %q, got %q", listing, out)
			}
		})
	}
}

// With --quiet, only a result is printed: not that
// something was changed, nor that nothing was found
func TestQuietCommand(t *testing.T) {
	for name, tc := range map[string]struct {
		args   []string
		method string
		path   string
		body   string
		stdout string
	}{
		"nothing found":  {[]string{"versions", "foo"}, "GET", "/packages/foo/versions", "[]", ""},
		"sharing add":    {[]string{"sharing", "add", "added@example.com"}, "PUT", "/collaborators/added@example.com", "{}", ""},
		"git rename":     {[]string{"git", "rename", "repo-name", "new-name"}, "PATCH", gitRepoPath, "{}", ""},
		"git destroy":    {[]string{"git", "destroy", "repo-name", "--force"}, "DELETE", gitRepoPath, "{}", ""},
		"git stack set":  {[]string{"git", "stack", "set", "repo-name", "fury-22"}, "PATCH", gitRepoPath, "{}", ""},
		"git config set": {[]string{"git", "config", "set", "repo-name", "KEY2=VALUE2"}, "PATCH", gitConfigPath, gitConfigResponse, ""},
		"git rebuild":    {[]string{"git", "rebuild", "repo-name"}, "POST", gitBuildsPath, gitRebuildResponse, gitRebuildResponse},
		"logout":         {[]string{"logout", "--yes"}, "POST", "/logout", "", ""},
		"login":          {[]string{"login", "--api-token", "abc123"}, "GET", "/users/me", whoamiResponse, ""},
		"push":           {[]string{"push", samplePackagePath()}, "POST", "/uploads", pushResponse, ""},
		"whoami":         {[]string{"whoami"}, "GET", "/users/me", whoamiResponse, "You are logged in as \"joetest\"\n"},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			server := testutil.APIServer(t, tc.method, tc.path, tc.body, 200)
			cc := testContext(t, term, auth, server)
			if err := runCommand(cc, append(tc.args, "--quiet")); err != nil {
				t.Fatal(err)
			}

			expectOutput(t, term, tc.stdout, "")
		})
	}
}

// With --quiet, a failure is reported as without. "push" has no status
// line to report a failed file by, so that is on stderr too.
func TestQuietFailure(t *testing.T) {
	const problem = "Problem uploading \"sample.txt\": " + uploadFailureMessage
	file := samplePackagePath()

	for name, tc := range map[string]struct {
		args   []string
		server func(*testing.T) *httptest.Server
		cause  error
		stderr string
	}{
		"forbidden":      {[]string{"packages"}, serving("GET", "/packages", 403, "[]"), api.ErrForbidden, "Error: You're not allowed to do this\n"},
		"push one file":  {[]string{"push", file}, failingUploadServer, api.ErrFuryServer, "Error: " + uploadFailureMessage},
		"push two files": {[]string{"push", file, file}, failingUploadServer, api.ErrFuryServer, problem + problem + "Error: 2 of 2 uploads failed\n"},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			cc := testContext(t, term, auth, tc.server(t))
			if err := runCommand(cc, append(tc.args, "-q")); !errors.Is(err, tc.cause) {
				t.Errorf("Expected %v within error, got: %v", tc.cause, err)
			}

			expectOutput(t, term, "", tc.stderr)
		})
	}
}

// With --quiet, a question is still asked, with the table of what it is about
func TestQuietYank(t *testing.T) {
	for name, tc := range map[string]struct {
		args  []string
		table bool
	}{
		"asked":   {[]string{"yank", "foo@1.2.3", "--quiet"}, true},
		"--force": {[]string{"yank", "foo@1.2.3", "--quiet", "--force"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()
			term.SetPromptResponses(map[string]string{
				yankConfirm: "y",
			})

			var removals atomic.Int32
			cc := testContext(t, term, auth, mutableServer(t, &removals))
			if err := runCommandNoErr(cc, tc.args); err != nil {
				t.Fatal(err)
			}

			out := string(term.OutBytes())
			if n := removals.Load(); n != 2 || strings.Contains(out, "Removed") {
				t.Errorf("Expected 2 removals that are not printed, got %d and %q", n, out)
			}
			if strings.Contains(out, "foo-1.2.3.tgz") != tc.table {
				t.Errorf("Expected table=%v, got %q", tc.table, out)
			}
		})
	}
}

// With --quiet, a download has no status line. A file that is there
// already is skipped in silence, unless it is stale, which is warned about.
func TestQuietDownload(t *testing.T) {
	for name, tc := range map[string]struct {
		present string // Content of the file beforehand, if any
		stderr  string
	}{
		"saved":   {"", ""},
		"skipped": {downloadContent, ""},
		"stale":   {staleContent, staleWarning},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			t.Chdir(t.TempDir())
			if tc.present != "" {
				if err := os.WriteFile("foo-1.2.3.tgz", []byte(tc.present), 0600); err != nil {
					t.Fatal(err)
				}
			}

			cc := testContext(t, term, auth, downloadServer(t))
			args := []string{"beta", "download", "foo@1.2.3", "--quiet", "--yes"}
			if err := runCommand(cc, args); err != nil {
				t.Fatal(err)
			}

			expectOutput(t, term, "", tc.stderr)
			if body, _ := os.ReadFile("foo-1.2.3.tgz"); string(body) != downloadContent {
				t.Errorf("Expected %q in the file, got %q", downloadContent, body)
			}
		})
	}
}
