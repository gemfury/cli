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
	path := "/packages/foo/versions/0.0.1"
	server := testutil.APIServer(t, "DELETE", path, "{}", 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	// Removing using version flag
	err := runCommandNoErr(cc, []string{"yank", "foo", "-v", "0.0.1"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "Removed package \"foo\" version \"0.0.1\"\n"
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	// Removing using PACKAGE@VERSION
	err = runCommandNoErr(cc, []string{"yank", "foo@0.0.1"})
	if err != nil {
		t.Fatal(err)
	}

	exp = "Removed package \"foo\" version \"0.0.1\"\n"
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
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
		mux.HandleFunc("/packages/foo/versions/0.0.1", func(w http.ResponseWriter, r *http.Request) {
			if method := r.Method; method != "DELETE" {
				t.Errorf("Invalid request: %s %s", method, r.URL.Path)
			}
			w.Write([]byte("{}"))
		})
		mux.HandleFunc("/packages/foo/versions/0.0.2", func(w http.ResponseWriter, r *http.Request) {
			if method := r.Method; method != "DELETE" {
				t.Errorf("Invalid request: %s %s", method, r.URL.Path)
			}
			http.NotFound(w, r)
		})
	})

	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	// Expected successful output
	exp := "Removed package \"foo\" version \"0.0.1\"\n"

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

	// Partial failure for multiple packages
	err = runCommandNoErr(cc, []string{"yank", "foo@0.0.1", "foo@0.0.2"})
	if err == nil || !strings.Contains(err.Error(), "Doesn't look like this exists") {
		t.Errorf("Expected invalid error, got %q", err)
	}
	if outStr := string(term.OutBytes()); !strings.Contains(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	// Success all around (reusing the same test package URL)
	err = runCommandNoErr(cc, []string{"yank", "foo@0.0.1", "foo@0.0.1"})
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestYankCommandUnauthorized(t *testing.T) {
	path := "/packages/foo/versions/0.0.1"
	server := testutil.APIServer(t, "DELETE", path, "{}", 200)
	testCommandLoginPreCheck(t, []string{"yank", "foo", "-v", "0.0.1"}, server)
	server.Close()
}

func TestYankCommandForbidden(t *testing.T) {
	path := "/packages/foo/versions/0.0.1"
	server := testutil.APIServer(t, "DELETE", path, "", 403)
	testCommandForbiddenResponse(t, []string{"yank", "foo", "-v", "0.0.1"}, server)
	server.Close()
}
