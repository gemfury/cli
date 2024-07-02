package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"
	"strings"
	"testing"
)

var packagesResponses = []string{`[{
	"id": "pkg_a1b2c3",
	"name": "pkg-ruby",
	"kind": "ruby",
	"private": false,
	"release_version": {
		"version": "1.1.1"
	}
}]`, `[{
	"id": "pkg_z1y2x3",
	"name": "pkg-js",
	"kind": "js",
	"private": true
}]`}

// ==== packages ====

func TestPackagesCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/packages"
	server := testutil.APIServerPaginated(t, "GET", path, packagesResponses, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
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
	server.Close()
}

func TestPackagesCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/packages", "[]", 403)
	testCommandForbiddenResponse(t, []string{"packages"}, server)
	server.Close()
}

// ==== versions ====

var versionsResponses = []string{`[{
	"id": "ver_a1b2c3",
	"version": "1.2.3",
	"created_at": "2011-05-27T00:39:07+00:00",
	"filename": "foo-1.2.3.tgz",
	"created_by": {
		"name": "user1"
	},
	"package": {
		"id": "pkg_x9y8z7",
		"kind": "js"
	}
}]`, `[{
	"id": "ver_z1y2x3",
	"version": "3.2.1",
	"created_at": "2011-01-27T00:44:00+00:00",
	"filename": "foo-3.2.1.tgz",
	"package": {
		"id": "pkg_x9y8z7",
		"kind": "js"
	}
}]`}

func TestVersionsCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/packages/pkg-name/versions"
	server := testutil.APIServerPaginated(t, "GET", path, versionsResponses, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"versions", "pkg-name"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "1.2.3 user1 2011-05-26 17:39 js foo-1.2.3.tgz 3.2.1 N/A 2011-01-26 16:44 js foo-3.2.1.tgz"
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestVersionsCommandUnauthorized(t *testing.T) {
	path := "/packages/pkg-name/versions"
	server := testutil.APIServer(t, "GET", path, "[]", 200)
	testCommandLoginPreCheck(t, []string{"versions", "pkg-name"}, server)
	server.Close()
}

func TestVersionsCommandForbidden(t *testing.T) {
	path := "/packages/pkg-name/versions"
	server := testutil.APIServer(t, "GET", path, "[]", 403)
	testCommandForbiddenResponse(t, []string{"versions", "pkg-name"}, server)
	server.Close()
}

// Some strings come from TabWriter with variable spacing
// Reduce multi-spaces to a single space for easier comparison
func compactString(b []byte) string {
	return strings.Join(strings.Fields(string(b)), " ")
}
