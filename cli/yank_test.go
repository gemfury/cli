package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"
	"strings"
	"testing"
)

// ==== YANK ====

func TestYankCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/packages/foo/versions/0.0.1"
	server := testutil.APIServer(t, "DELETE", path, "{}", 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"yank", "foo", "-v", "0.0.1"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "Removed package \"foo\" version \"0.0.1\"\n"
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
