package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// The second package has a prerelease alone, and so no release
var packagesResponses = []string{`[{
	"id": "pkg_a1b2c3",
	"name": "pkg-ruby",
	"kind_key": "ruby",
	"private": false,
	"version_count": 3,
	"latest_version": {
		"id": "ver_r1r2r3",
		"version": "1.2.0.pre",
		"prerelease": true
	},
	"release_version": {
		"id": "ver_q1q2q3",
		"version": "1.1.1",
		"prerelease": false
	}
}]`, `[{
	"id": "pkg_z1y2x3",
	"name": "pkg-js",
	"kind_key": "js",
	"private": true,
	"version_count": 1,
	"latest_version": {
		"id": "ver_j1j2j3",
		"version": "0.1.0-beta.1",
		"prerelease": true
	},
	"release_version": null
}]`}

// ==== packages ====

func TestPackagesCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/packages"
	server := testutil.APIServerPaginated(t, "GET", path, packagesResponses, 200)

	cc := cli.TestContext(t.Context(), term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"packages"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "pkg-ruby ruby 1.1.1 public pkg-js js beta private"
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestPackagesCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/packages", "[]", 200)
	testCommandLoginPreCheck(t, []string{"packages"}, server)
}

func TestPackagesCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/packages", "[]", 403)
	testCommandForbiddenResponse(t, []string{"packages"}, server, "")
}

// ==== versions ====

// The second version has no "download_url", as it is not downloadable
var versionsResponses = []string{`[{
	"id": "ver_a1b2c3",
	"version": "1.2.3",
	"kind_key": "js",
	"digests": {
		"md5": "0123abcd",
		"sha1": "4567ef01",
		"sha256": "89ab2345",
		"sha512": "cdef6789"
	},
	"filename": "foo-1.2.3.tgz",
	"download_url": "https://api.fury.io/2/indexes/js/user/download/ver_a1b2c3/foo-1.2.3",
	"prerelease": false,
	"created_at": "2011-05-27T00:39:07+00:00",
	"created_by": {
		"id": "acct_d4e5f6",
		"name": "user1"
	},
	"package": {
		"id": "pkg_x9y8z7",
		"name": "foo",
		"kind_key": "js",
		"private": true,
		"version_count": 2
	}
}]`, `[{
	"id": "ver_z1y2x3",
	"version": "3.2.1",
	"kind_key": "js",
	"digests": {
		"md5": "fedc3210",
		"sha1": "ba987654",
		"sha256": "76543210",
		"sha512": "3210fedc"
	},
	"filename": "foo-3.2.1.tgz",
	"prerelease": false,
	"created_at": "2011-01-27T00:44:00+00:00",
	"created_by": {
		"id": "acct_g7h8i9",
		"name": "user2"
	},
	"package": {
		"id": "pkg_x9y8z7",
		"name": "foo",
		"kind_key": "js",
		"private": true,
		"version_count": 2
	}
}]`}

func TestVersionsCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/packages/pkg-name/versions"
	server := testutil.APIServerPaginated(t, "GET", path, versionsResponses, 200)

	cc := cli.TestContext(t.Context(), term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"versions", "pkg-name"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "1.2.3 user1 2011-05-26 17:39 js foo-1.2.3.tgz 3.2.1 user2 2011-01-26 16:44 js foo-3.2.1.tgz"
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

// A version without an uploader or a kind is listed with N/A for them
func TestVersionsCommandUnknownFields(t *testing.T) {
	term := terminal.NewForTest()
	body := `[{"version": "1.0.0", "filename": "foo-1.0.0.tgz", "created_at": "2011-01-27T00:44:00+00:00"}]`
	server := testutil.APIServer(t, "GET", "/packages/pkg-name/versions", body, 200)

	cc := testContext(t, term, terminal.TestAuther("user", "abc123", nil), server)
	if err := runCommand(cc, []string{"versions", "pkg-name"}); err != nil {
		t.Fatal(err)
	}

	exp := "1.0.0 N/A 2011-01-26 16:44 N/A foo-1.0.0.tgz"
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestVersionsCommandEmpty(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	path := "/packages/pkg-name/versions"
	server := testutil.APIServer(t, "GET", path, "[]", 200)

	cc := testContext(t, term, auth, server)
	if err := runCommand(cc, []string{"versions", "pkg-name"}); err != nil {
		t.Fatal(err)
	}

	expectOutput(t, term, "No versions found for package \"pkg-name\"\n", "")
}

func TestVersionsCommandUnauthorized(t *testing.T) {
	path := "/packages/pkg-name/versions"
	server := testutil.APIServer(t, "GET", path, "[]", 200)
	testCommandLoginPreCheck(t, []string{"versions", "pkg-name"}, server)
}

func TestVersionsCommandForbidden(t *testing.T) {
	path := "/packages/pkg-name/versions"
	server := testutil.APIServer(t, "GET", path, "[]", 403)
	testCommandForbiddenResponse(t, []string{"versions", "pkg-name"}, server, `Package "pkg-name"`)
}

// Some strings come from TabWriter with variable spacing
// Reduce multi-spaces to a single space for easier comparison
func compactString(b []byte) string {
	return strings.Join(strings.Fields(string(b)), " ")
}

// A listing interrupted between pages is incomplete, so it must fail
func TestPackagesCommandCancelled(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	cc, cancel := context.WithCancel(cli.TestContext(t.Context(), term, auth))
	defer cancel()

	var requests atomic.Int32
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/packages", func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			cancel() // Interrupted while the first page is in flight
			testutil.APIPaginatedResponse(t, w, r, packagesResponses, 200)
		})
	})

	ctx.GlobalFlags(cc).Endpoint = server.URL

	err := runCommand(cc, []string{"packages"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}

	if n := requests.Load(); n != 1 {
		t.Errorf("Expected 1 request before cancellation, got %d", n)
	}
}

// A pagination cursor that repeats would list forever, so it must fail
func TestPackagesCommandRepeatedCursor(t *testing.T) {
	cursors := []string{"a", "b", "a"} // The next page of each request
	var requests atomic.Int32
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/packages", func(w http.ResponseWriter, r *http.Request) {
			i := int(requests.Add(1)) - 1
			if i >= len(cursors) {
				w.WriteHeader(500)
				return
			}
			w.Header().Set("Link", "</packages?page="+cursors[i]+">; rel=\"next\"")
			w.Write([]byte(packagesResponses[0]))
		})
	})

	term := terminal.NewForTest()
	cc := testContext(t, term, terminal.TestAuther("user", "abc123", nil), server)
	err := runCommand(cc, []string{"packages"})
	if err == nil || !strings.Contains(err.Error(), "Repeated pagination cursor") {
		t.Fatalf("Expected cursor error, got %v", err)
	}
	if n := requests.Load(); int(n) != len(cursors) {
		t.Fatalf("Expected %d requests before the repeat, got %d", len(cursors), n)
	}
}

// An unusable endpoint is reported as an error by every kind of request
func TestCommandInvalidEndpoint(t *testing.T) {
	for _, args := range [][]string{
		{"packages"},
		{"push", samplePackagePath()},
		{"git", "config", "set", "repo", "A=1"},
	} {
		t.Run(args[0], func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			cc := cli.TestContext(t.Context(), terminal.NewForTest(), auth)

			flags := ctx.GlobalFlags(cc)
			flags.PushEndpoint = "://invalid"
			flags.Endpoint = "://invalid"

			if err := runCommand(cc, args); err == nil {
				t.Error("Expected an error for an invalid endpoint")
			}
		})
	}
}
