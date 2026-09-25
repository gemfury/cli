package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"testing"
)

// ==== beta download ====

// A malformed argument and a missing version: both are reported, in order
func TestDownloadCommandFailures(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	server := testutil.APIServer(t, "GET", "/packages/foo/versions/1.2.3", "", 404)
	defer server.Close()

	cc := testContext(term, auth, server)
	err := runCommand(cc, []string{"beta", "download", "no-version", "foo@1.2.3"})
	expectSummaryError(t, err, api.ErrNotFound, "2 of 2 downloads failed")
	expectProblems(t, term,
		"Problem downloading \"no-version\": Argument format: PACKAGE@VERSION\n",
		"Problem downloading \"foo@1.2.3\": Doesn't look like this exists\n",
	)
}
