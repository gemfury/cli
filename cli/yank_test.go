package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// ==== YANK ====

// What "yank" asks before it removes anything
const yankConfirm = "Are you sure you want to delete these files? [y/N]"

func TestYankCommandOnePackage(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/versions", func(w http.ResponseWriter, r *http.Request) {
			if q := r.URL.Query(); q.Get("name") != "foo" || q.Get("version") != "0.0.1" {
				t.Errorf("Invalid request: %s %s", r.Method, r.URL.Path)
			} else if k := q.Get("kind"); k == "js" {
				w.Write([]byte(versionsResponses[0])) // One page
				return
			} else if method := r.Method; method != "GET" {
				t.Errorf("Invalid method: %s %s", method, r.URL.Path)
			}
			testutil.APIPaginatedResponse(t, w, r, versionsResponses, 200)
		})
		mux.HandleFunc("/packages/{pid}/versions/{vid}", func(w http.ResponseWriter, r *http.Request) {
			if method := r.Method; method != "DELETE" {
				t.Errorf("Invalid request: %s %s", method, r.URL.Path)
				w.WriteHeader(500)
			}
			w.Write([]byte("{}"))
		})
	})

	cc := cli.TestContext(t.Context(), term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	// Removing using version flag
	err := runCommandNoErr(cc, []string{"yank", "foo", "-v", "0.0.1", "--force"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "Removed \"foo-1.2.3.tgz\"\nRemoved \"foo-3.2.1.tgz\"\n"
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	// Removing using PACKAGE@VERSION
	err = runCommandNoErr(cc, []string{"yank", "foo@0.0.1", "--force"})
	if err != nil {
		t.Fatal(err)
	} else if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	// Removing using KIND:PACKAGE@VERSION
	exp = "Removed \"foo-1.2.3.tgz\"\n" // JS kind returns one Version
	err = runCommandNoErr(cc, []string{"yank", "js:foo@0.0.1", "--force"})
	if err != nil {
		t.Fatal(err)
	} else if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestYankCommandMultiPackage(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/versions", func(w http.ResponseWriter, r *http.Request) {
			if q := r.URL.Query(); q.Get("name") != "foo" {
				t.Errorf("Invalid name: %s %s", r.Method, r.URL.Path)
			} else if v := q.Get("version"); v == "0.0.2" {
				w.Write([]byte("[]")) // Nothing found
			} else if v != "0.0.1" {
				t.Errorf("Invalid version: %s %s", r.Method, r.URL.Path)
			} else if method := r.Method; method != "GET" {
				t.Errorf("Invalid method: %s %s", method, r.URL.Path)
			} else {
				testutil.APIPaginatedResponse(t, w, r, versionsResponses, 200)
			}
		})
		mux.HandleFunc("/packages/{pid}/versions/{vid}", func(w http.ResponseWriter, r *http.Request) {
			if method := r.Method; method != "DELETE" {
				t.Errorf("Invalid request: %s %s", method, r.URL.Path)
				w.WriteHeader(500)
			}
			w.Write([]byte("{}"))
		})
	})

	cc := cli.TestContext(t.Context(), term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	// Expected successful output
	exp := "Removed \"foo-1.2.3.tgz\"\nRemoved \"foo-3.2.1.tgz\"\n"

	// When nothing is found, we expect "nothing found" error message
	expNone := "No matching versions found\n"
	if err := runCommand(cc, []string{"yank", "foo@0.0.2", "--force"}); err != nil {
		t.Error(err)
	}
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, expNone) {
		t.Errorf("Expected output to include %q, got %q", expNone, outStr)
	}

	// No partial failure for multiple packages when some return nothing
	if err := runCommand(cc, []string{"yank", "foo@0.0.1", "foo@0.0.2", "--force"}); err != nil {
		t.Error(err)
	}
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	// Success all around (reusing the same test package URL)
	if err := runCommand(cc, []string{"yank", "foo@0.0.1", "foo@0.0.1", "--force"}); err != nil {
		t.Error(err)
	}
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	// Success all around with confirmation prompt
	term.SetPromptResponses(map[string]string{
		yankConfirm: "Y",
	})

	if err := runCommand(cc, []string{"yank", "foo@0.0.1"}); err != nil {
		t.Error(err)
	}
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

// Nothing is removed unless confirmed
func TestYankCommandUnconfirmed(t *testing.T) {
	for answer, interrupted := range unconfirmed {
		t.Run(answer, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/versions", func(w http.ResponseWriter, r *http.Request) {
					testutil.APIPaginatedResponse(t, w, r, versionsResponses, 200)
				})
				mux.HandleFunc("/packages/{pid}/versions/{vid}", func(w http.ResponseWriter, r *http.Request) {
					t.Errorf("Unexpected removal: %s %s", r.Method, r.URL.Path)
				})
			})

			term.SetPromptResponses(map[string]string{
				yankConfirm: answer,
			})

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, []string{"yank", "foo@0.0.1"})
			expectUnconfirmed(t, term, err, interrupted)
		})
	}
}

// Interrupted while removing the first of two versions
func TestYankCommandInterrupted(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	cc, cancel := context.WithCancel(cli.TestContext(t.Context(), term, auth))
	defer cancel()

	var removals atomic.Int32
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/versions", func(w http.ResponseWriter, r *http.Request) {
			testutil.APIPaginatedResponse(t, w, r, versionsResponses, 200)
		})
		mux.HandleFunc("/packages/{pid}/versions/{vid}", interruptOn(cancel, &removals))
	})

	ctx.GlobalFlags(cc).Endpoint = server.URL

	expectInterrupted(t, runCommand(cc, []string{"yank", "foo@0.0.1", "--force"}))
	if n := removals.Load(); n != 1 {
		t.Errorf("Expected 1 removal before interruption, got %d", n)
	}

	expectOutput(t, term, "", "Cancelled\n")
}

func TestYankCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/versions", "[]", 200)
	testCommandLoginPreCheck(t, []string{"yank", "foo", "-v", "0.0.1"}, server)
}

func TestYankCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/versions", "", 403)
	testCommandForbiddenResponse(t, []string{"yank", "foo", "-v", "0.0.1"}, server, `Version "foo@0.0.1"`)
}

// Versions that cannot be looked up: each is reported, then summarized.
// The version follows the last "@", as the name of a package may have one.
func TestYankCommandLookupFailures(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	server := testutil.APIServer(t, "GET", "/versions", "", 404)

	cc := testContext(t, term, auth, server)
	err := runCommand(cc, []string{"yank", "foo@1.0", "@scope/bar@1.0", "--force"})
	expectSummaryError(t, err, api.ErrNotFound, "2 of 2 lookups failed")
	expectOutput(t, term, "", ""+
		"Problem looking up \"foo@1.0\": Doesn't look like this exists\n"+
		"Problem looking up \"@scope/bar@1.0\": Doesn't look like this exists\n"+
		"Error: 2 of 2 lookups failed\n")
}

// A single failure is the error of the command, about what it failed on:
// the version that is looked up, or the file that is removed
func TestYankCommandSingleFailure(t *testing.T) {
	for name, tc := range map[string]struct {
		lookup  http.HandlerFunc
		message string
	}{
		"lookup": {
			respondWith(404, ""),
			"Version \"js:foo@0.0.1\": Doesn't look like this exists",
		},
		"removal": {
			respondWith(200, versionsResponses[0]),
			"File \"foo-1.2.3.tgz\": Doesn't look like this exists",
		},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("GET /versions", tc.lookup)
				mux.HandleFunc("DELETE /packages/{pid}/versions/{vid}", respondWith(404, ""))
			})

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, []string{"yank", "js:foo@0.0.1", "--force"})
			expectExitStatus(t, term, err, cli.ExitNotFound)
			expectOutput(t, term, "", "Error: "+tc.message+"\n")
		})
	}
}

// One of two matched versions cannot be removed: the other still is
func TestYankCommandPartialRemovalFailure(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/versions", func(w http.ResponseWriter, r *http.Request) {
			testutil.APIPaginatedResponse(t, w, r, versionsResponses, 200)
		})
		mux.HandleFunc("/packages/{pid}/versions/{vid}", func(w http.ResponseWriter, r *http.Request) {
			if r.PathValue("vid") == "ver_z1y2x3" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write([]byte("{}"))
		})
	})

	cc := testContext(t, term, auth, server)
	err := runCommand(cc, []string{"yank", "foo@0.0.1", "--force"})
	expectSummaryError(t, err, api.ErrNotFound, "1 of 2 removals failed")

	expectOutputLines(t, term, "Removed ", "Removed \"foo-1.2.3.tgz\"\n")
	expectProblems(t, term, "Problem removing \"foo-3.2.1.tgz\": Doesn't look like this exists\n")
}
