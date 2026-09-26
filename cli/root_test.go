package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Neither usage text nor "nothing found" belongs in a failed command's stdout
var failedOutRegexp = regexp.MustCompile(`(?m)^(Usage:|No .* found)`)

// Top-level testing initializer
func TestMain(m *testing.M) {
	os.Setenv("TZ", "US/Pacific")
	os.Exit(m.Run())
}

func TestRootCommand(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Fire up test server (error on everything)
	server := testutil.APIServer(t, "", "/", "", 501)
	defer server.Close()

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	if err := runCommandNoErr(cc, []string{""}); err != nil {
		t.Fatal(err)
	}

	outStr := string(term.OutBytes())
	if exp := "See https://gemfury.com/help/gemfury-cli\n"; !strings.HasPrefix(outStr, exp) {
		t.Errorf("Expected output to start with %q, got %q", exp, outStr)
	}
}

func runCommand(cc context.Context, args []string) error {
	cmd := cli.NewRootCommand(cc)
	cmd.SetArgs(args)
	return cli.Execute(cc, cmd)
}

func runCommandNoErr(cc context.Context, args []string) error {
	if err := runCommand(cc, args); err != nil {
		return fmt.Errorf("Command error: %w", err)
	}

	term := ctx.TestTerm(cc)
	if errStr := string(term.ErrBytes()); errStr != "" {
		return fmt.Errorf("Error output: %q", errStr)
	}

	return nil
}

// testContext builds a command context with both API endpoints pointed at server
func testContext(term terminal.Terminal, auth terminal.Auther, server *httptest.Server, opts ...testOption) context.Context {
	cc := cli.TestContext(term, auth)
	for _, opt := range opts {
		cc = opt(cc)
	}

	flags := ctx.GlobalFlags(cc)
	flags.PushEndpoint = server.URL
	flags.Endpoint = server.URL
	return cc
}

// expectSummaryError asserts the error a multi-item command returns when
// some items failed: it reads as summary and still wraps cause
func expectSummaryError(t *testing.T, err, cause error, summary string) {
	t.Helper()
	if !errors.Is(err, cause) {
		t.Fatalf("Expected %v within error, got: %v", cause, err)
	}
	if err.Error() != summary {
		t.Errorf("Expected summary %q, got %q", summary, err)
	}
}

// expectProblems asserts that stderr begins with the given lines, in order
func expectProblems(t *testing.T, term terminal.TestTerm, lines ...string) {
	t.Helper()
	errStr := string(term.ErrBytes())
	if exp := strings.Join(lines, ""); !strings.HasPrefix(errStr, exp) {
		t.Errorf("Error output should start with %q, got %q", exp, errStr)
	}
}

// expectOutput asserts exactly what went to stdout and to stderr
func expectOutput(t *testing.T, term terminal.TestTerm, stdout, stderr string) {
	t.Helper()
	if out := string(term.OutBytes()); out != stdout {
		t.Errorf("Output should be %q, got %q", stdout, out)
	}
	if errOut := string(term.ErrBytes()); errOut != stderr {
		t.Errorf("Error output should be %q, got %q", stderr, errOut)
	}
}

// expectOutputLines asserts that stdout holds the given adjacent lines and no
// other occurrence of marker. Stdout is not matched exactly because it may
// also hold other progress or status lines.
func expectOutputLines(t *testing.T, term terminal.TestTerm, marker string, lines ...string) {
	t.Helper()
	out := string(term.OutBytes())
	if exp := strings.Join(lines, ""); !strings.Contains(out, exp) || strings.Count(out, marker) != len(lines) {
		t.Errorf("Output should contain exactly %q, got %q", exp, out)
	}
}

// offlineServer fails the test on any request. Registering "/" also
// suppresses testutil's default browser-login handler, so a command that
// wrongly attempts login fails loudly instead of quietly succeeding.
func offlineServer(t *testing.T) *httptest.Server {
	return testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("Unexpected API request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotImplemented)
		})
	})
}

// We first test with manual (prompt) login, and then test with "--api-token" flag
func testCommandLoginPreCheck(t *testing.T, args []string, server *httptest.Server, opts ...testOption) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()
	cc := testContext(term, auth, server, opts...)

	// Prepare for browser login prompt
	term.InWrite([]byte("!"))

	if err := runCommand(cc, args); err != nil {
		t.Errorf("Command error: %s", err)
	}

	if u, p, err := auth.Auth(); err != nil {
		t.Errorf("Login error: %s", err)
	} else if exp := "u@example.com"; u != exp {
		t.Errorf("Expected user %q, got %q", exp, u)
	} else if exp := "token-abc-123"; p != exp {
		t.Errorf("Expected pass %q, got %q", exp, p)
	}

	// Testing with "--api-token" should skip calling Auth() on TestAuther.
	// The context options only shape the logged-out run above.
	auth = terminal.TestAuther("", "", fmt.Errorf("TestAuther should not be called"))
	cc = testContext(term, auth, server)

	args = append(args, "--api-token", "abc123")
	if err := runCommand(cc, args); err != nil {
		t.Errorf("Command error: %s", err)
	}
}

func testCommandForbiddenResponse(t *testing.T, args []string, server *httptest.Server, opts ...testOption) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()
	cc := testContext(term, auth, server, opts...)

	err := runCommand(cc, args)
	if !errors.Is(err, api.ErrForbidden) {
		t.Fatalf("Command error: %s", err)
	}

	// Error is reported exactly once, on stderr, without usage text
	errStr := string(term.ErrBytes())
	if exp := "Error: You're not allowed to do this\n"; errStr != exp {
		t.Errorf("Error should be %q, got %q", exp, errStr)
	}

	if ob := term.OutBytes(); failedOutRegexp.Match(ob) {
		t.Errorf("Unexpected output for an API error: %q", ob)
	}
}

// Help never requires authentication, and never contacts the API
func TestHelpWithoutAuth(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"help", "push"}, {"git", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			server := offlineServer(t)
			defer server.Close()

			term := terminal.NewForTest()
			cc := testContext(term, terminal.TestAuther("", "", nil), server)
			if err := runCommandNoErr(cc, args); err != nil {
				t.Error(err)
			}

			if outStr := string(term.OutBytes()); !strings.Contains(outStr, "Usage:") {
				t.Errorf("Expected help on stdout, got %q", outStr)
			}
		})
	}
}

// Wrong arguments or flags are usage errors, caught before authentication,
// so a logged-out user is not sent to log in first
func TestUsageErrorOutput(t *testing.T) {
	cases := []struct {
		args []string
		msg  string
	}{
		{[]string{"versions", "one", "two"}, "Please specify exactly one package"},
		{[]string{"push"}, "Please specify at least one package file"},
		{[]string{"yank"}, "Please specify at least one package"},
		{[]string{"yank", "foo", "bar", "-v", "0.0.1"}, "Use PACKAGE@VERSION for multiple yanks"},
		{[]string{"beta", "download"}, "Please specify at least one PACKAGE@VERSION"},
		{[]string{"beta", "backup"}, "Please specify exactly one destination directory"},
		{[]string{"git", "rename", "repo"}, "Please specify a repository and its new name"},
		{[]string{"git", "config", "get", "repo"}, "Please specify a repository and at least one key"},
		{[]string{"git", "config", "set", "repo", "A=1", "B"}, "Argument has no value: B"},
		{[]string{"git", "stack", "set", "repo"}, "Please specify a repository and a stack"},
		{[]string{"sharing", "add"}, "Please specify at least one collaborator"},
		{[]string{"sharing", "extra"}, `unknown command "extra" for "fury sharing"`},
		{[]string{"whoami", "extra"}, `unknown command "extra" for "fury whoami"`},
		{[]string{"logout", "now"}, `unknown command "now" for "fury logout"`},
		{[]string{"packages", "--json"}, "unknown flag: --json"},
	}

	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			server := offlineServer(t)
			defer server.Close()

			term := terminal.NewForTest()
			cc := testContext(term, terminal.TestAuther("", "", nil), server)

			err := runCommand(cc, tc.args)
			if !cli.IsUsageError(err) {
				t.Fatalf("Expected usage error, got: %v", err)
			}

			// Error and usage both go to stderr: the error once, then usage
			expectProblems(t, term, "Error: "+tc.msg+"\n", "Usage:\n")

			if ob := term.OutBytes(); len(ob) != 0 {
				t.Errorf("Expected empty stdout, got %q", ob)
			}
		})
	}
}

// Without Args, Cobra lets a subcommand silently ignore extra arguments
func TestRunnableCommandsDeclareArgs(t *testing.T) {
	cc := cli.TestContext(terminal.NewForTest(), terminal.TestAuther("", "", nil))

	var check func(cmd *cobra.Command)
	check = func(cmd *cobra.Command) {
		if cmd.Runnable() && cmd.Args == nil {
			t.Errorf("Command %q has no Args check", cmd.CommandPath())
		}
		for _, sub := range cmd.Commands() {
			check(sub)
		}
	}
	check(cli.NewRootCommand(cc))
}

func TestUnknownCommandOutput(t *testing.T) {
	server := offlineServer(t)
	defer server.Close()

	term := terminal.NewForTest()
	cc := testContext(term, terminal.TestAuther("", "", nil), server)
	if err := runCommand(cc, []string{"nosuchcmd"}); err == nil {
		t.Fatal("Expected error for unknown command")
	}

	expectOutput(t, term, "", "Error: unknown command \"nosuchcmd\" for \"fury\"\nRun 'fury --help' for usage.\n")
}

// Context altering options added to test commands
type testOption func(context.Context) context.Context

// No login option
func noLoginOpt(cc context.Context) context.Context {
	ctx.Auther(cc).Wipe()
	return cc
}
