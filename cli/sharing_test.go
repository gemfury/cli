package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"
	"strings"
	"testing"
)

// ==== sharing remove ====

func TestSharingRemoveCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/collaborators/fired@example.com"
	server := testutil.APIServer(t, "DELETE", path, "{}", 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"sharing", "remove", "fired@example.com"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "Removed \"fired@example.com\" as a collaborator\n"
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestSharingRemoveCommandUnauthorized(t *testing.T) {
	path := "/collaborators/fired@example.com"
	server := testutil.APIServer(t, "DELETE", path, "{}", 200)
	args := []string{"sharing", "remove", "fired@example.com"}
	testCommandLoginPreCheck(t, args, server)
	server.Close()
}

func TestSharingRemoveForbidden(t *testing.T) {
	path := "/collaborators/fired@example.com"
	server := testutil.APIServer(t, "DELETE", path, "{}", 403)
	args := []string{"sharing", "remove", "fired@example.com"}
	testCommandForbiddenResponse(t, args, server)
	server.Close()
}
