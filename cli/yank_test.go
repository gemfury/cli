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

// ==== YANK ====

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
	defer server.Close()

	cc := cli.TestContext(term, auth)
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

	// Fail if no version specified
	err = runCommandNoErr(cc, []string{"yank", "foo"})
	if err == nil || !strings.Contains(err.Error(), "Invalid package/version") {
		t.Errorf("Expected invalid error, got %q", err)
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
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	// Expected successful output
	exp := "Removed \"foo-1.2.3.tgz\"\nRemoved \"foo-3.2.1.tgz\"\n"

	// Failure for multiple packages without version
	err := runCommandNoErr(cc, []string{"yank", "foo", "bar"})
	if err == nil || !strings.Contains(err.Error(), "Invalid package/version") {
		t.Errorf("Expected invalid error, got %q", err)
	}

	// Failure for multiple packages with version flag
	err = runCommandNoErr(cc, []string{"yank", "foo", "bar", "-v", "0.0.1"})
	if err == nil || !strings.Contains(err.Error(), "Use PACKAGE@VERSION") {
		t.Errorf("Expected invalid error, got %q", err)
	}

	// When nothing is found, we expect "nothing found" error message
	expNone := "No matching versions found\n"
	err = runCommandNoErr(cc, []string{"yank", "foo@0.0.2", "--force"})
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, expNone) {
		t.Errorf("Expected output to include %q, got %q", expNone, outStr)
	}

	// No partial failure for multiple packages when some return nothing
	err = runCommandNoErr(cc, []string{"yank", "foo@0.0.1", "foo@0.0.2", "--force"})
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	// Success all around (reusing the same test package URL)
	err = runCommandNoErr(cc, []string{"yank", "foo@0.0.1", "foo@0.0.1", "--force"})
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	// Success all around with confirmation prompt
	term.SetPromptResponses(map[string]string{
		"Are you sure you want to delete these files? [y/N]": "Y",
	})

	err = runCommandNoErr(cc, []string{"yank", "foo@0.0.1"})
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestYankCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/versions", "[]", 200)
	testCommandLoginPreCheck(t, []string{"yank", "foo", "-v", "0.0.1"}, server)
	server.Close()
}

func TestYankCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/versions", "", 403)
	testCommandForbiddenResponse(t, []string{"yank", "foo", "-v", "0.0.1"}, server)
	server.Close()
}
