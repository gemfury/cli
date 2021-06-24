package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"
	"strings"
	"testing"
)

var sharingResponses = []string{`[{
	"id": "acct_a1b2c3",
	"name": "test-name",
	"username": "test-user",
	"role": "owner"
}]`, `[{
	"id": "acct_z1y2x3",
	"name": "collaborator",
	"username": "test-collab",
	"role": "push"
}]`}

// ==== sharing ====

func TestSharingCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/members"
	server := testutil.APIServerPaginated(t, "GET", path, sharingResponses, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"sharing"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "test-name     owner\ncollaborator  push\n"
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestSharingCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/members", "[]", 200)
	testCommandLoginPreCheck(t, []string{"sharing"}, server)
	server.Close()
}

func TestSharingCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/members", "[]", 403)
	testCommandForbiddenResponse(t, []string{"sharing"}, server)
	server.Close()
}

// ==== sharing add ====

func TestSharingAddCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server
	path := "/collaborators/added@example.com"
	server := testutil.APIServer(t, "PUT", path, "{}", 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"sharing", "add", "added@example.com"})
	if err != nil {
		t.Fatal(err)
	}

	exp := "Invited \"added@example.com\" as a collaborator\n"
	if outStr := string(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestSharingAddCommandUnauthorized(t *testing.T) {
	path := "/collaborators/added@example.com"
	server := testutil.APIServer(t, "PUT", path, "{}", 200)
	args := []string{"sharing", "add", "added@example.com"}
	testCommandLoginPreCheck(t, args, server)
	server.Close()
}

func TestSharingAddForbidden(t *testing.T) {
	path := "/collaborators/added@example.com"
	server := testutil.APIServer(t, "PUT", path, "{}", 403)
	args := []string{"sharing", "add", "added@example.com"}
	testCommandForbiddenResponse(t, args, server)
	server.Close()
}

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
