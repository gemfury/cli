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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// Neither usage text nor "nothing found" belongs in a failed command's stdout
var failedOutRegexp = regexp.MustCompile(`(?m)^(Usage:|No .* found)`)

// Top-level testing initializer
func TestMain(m *testing.M) {
	os.Setenv("TZ", "US/Pacific")

	// Tests run the same whatever the developer's own Gemfury setup
	os.Unsetenv("FURY_TOKEN")
	os.Unsetenv("FURY_ACCOUNT")

	os.Exit(m.Run())
}

// Without a command, the root shows its help
func TestRootCommand(t *testing.T) {
	if out, exp := helpOutput(t, ""), helpOutput(t, "--help"); out != exp {
		t.Errorf("Expected output to be %q, got %q", exp, out)
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

// helpOutput runs a command that only shows help, and returns its stdout.
// It has no credentials, and any API request fails the test.
func helpOutput(t *testing.T, args ...string) string {
	t.Helper()
	term := terminal.NewForTest()
	cc := testContext(t, term, terminal.TestAuther("", "", nil), offlineServer(t))
	if err := runCommandNoErr(cc, args); err != nil {
		t.Fatal(err)
	}
	return string(term.OutBytes())
}

// allCommands lists cmd and every command below it
func allCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{cmd}
	for _, sub := range cmd.Commands() {
		cmds = append(cmds, allCommands(sub)...)
	}
	return cmds
}

// testContext builds a command context with both API endpoints pointed at server
func testContext(t *testing.T, term terminal.Terminal, auth terminal.Auther, server *httptest.Server, opts ...testOption) context.Context {
	t.Helper()
	cc := cli.TestContext(t.Context(), term, auth)
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
	expectErrOutput(t, term, stderr)
}

// expectErrOutput asserts exactly what went to stderr, whatever went to stdout
func expectErrOutput(t *testing.T, term terminal.TestTerm, stderr string) {
	t.Helper()
	if errOut := string(term.ErrBytes()); errOut != stderr {
		t.Errorf("Error output should be %q, got %q", stderr, errOut)
	}
}

// expectExitStatus asserts the exit status for the error of a command
func expectExitStatus(t *testing.T, term terminal.TestTerm, err error, exp int) {
	t.Helper()
	if got := cli.ExitStatus(err); got != exp {
		t.Errorf("Expected exit status %d, got %d with %q", exp, got, term.ErrBytes())
	}
}

// expectCredentials asserts what credentials are saved, both empty for none
func expectCredentials(t *testing.T, auth terminal.Auther, user, token string) {
	t.Helper()
	if u, p, _ := auth.Auth(); u != user || p != token {
		t.Errorf("Expected saved credentials %q/%q, got %q/%q", user, token, u, p)
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
	t.Helper()
	return testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("Unexpected API request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotImplemented)
		})
	})
}

// respondWith handles any request with the given status and body
func respondWith(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}
}

// unusedAuther fails any command that consults the saved credentials
func unusedAuther() terminal.Auther {
	return terminal.TestAuther("", "", errors.New("TestAuther should not be called"))
}

// We first test with manual (prompt) login, and then test with "--api-token" flag
func testCommandLoginPreCheck(t *testing.T, args []string, server *httptest.Server, opts ...testOption) {
	t.Helper()
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()
	cc := testContext(t, term, auth, server, opts...)

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
	cc = testContext(t, term, unusedAuther(), server)

	args = append(args, "--api-token", "abc123")
	if err := runCommand(cc, args); err != nil {
		t.Errorf("Command error: %s", err)
	}
}

// testCommandForbiddenResponse asserts how a command reports a 403: about
// what it asked for, e.g. `Package "foo"`, or about nothing when empty
func testCommandForbiddenResponse(t *testing.T, args []string, server *httptest.Server, about string) {
	t.Helper()
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()
	cc := testContext(t, term, auth, server)

	err := runCommand(cc, args)
	if !errors.Is(err, api.ErrForbidden) {
		t.Fatalf("Command error: %s", err)
	}

	if about != "" {
		about += ": "
	}

	// Error is reported exactly once, on stderr, without usage text
	expectErrOutput(t, term, "Error: "+about+"You're not allowed to do this\n")

	if ob := term.OutBytes(); failedOutRegexp.Match(ob) {
		t.Errorf("Unexpected output for an API error: %q", ob)
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
		{[]string{"yank", "foo", "bar"}, "Invalid package/version specified: foo"},
		{[]string{"yank", "foo@1.0", "bar@"}, "Invalid package/version specified: bar@"},
		{[]string{"yank", "", "-v", "1.0"}, "Invalid package/version specified: "},
		{[]string{"yank", "js:@1.0"}, "Invalid package/version specified: js:@1.0"},
		{[]string{"yank", "js:", "-v", "1.0"}, "Invalid package/version specified: js:"},
		{[]string{"beta", "download"}, "Please specify at least one PACKAGE@VERSION"},
		{[]string{"beta", "download", "foo@1.0", "@1.0"}, "Argument format is PACKAGE@VERSION: @1.0"},
		{[]string{"beta", "backup"}, "Please specify exactly one destination directory"},
		{[]string{"git", "rename", "repo"}, "Please specify a repository and its new name"},
		{[]string{"git", "config", "get", "repo"}, "Please specify a repository and at least one key"},
		{[]string{"git", "config", "set", "repo", "A=1", "B"}, "Argument has no value: B"},
		{[]string{"git", "stack", "set", "repo"}, "Please specify a repository and a stack"},
		{[]string{"git", "credentials"}, "Please specify exactly one operation"},
		{[]string{"sharing", "add"}, "Please specify at least one collaborator"},
		{[]string{"git", "nosuch"}, `unknown command "nosuch" for "fury git"`},
		{[]string{"beta", "nosuch"}, `unknown command "nosuch" for "fury beta"`},
		{[]string{"sharing", "extra"}, `unknown command "extra" for "fury sharing"`},
		{[]string{"whoami", "extra"}, `unknown command "extra" for "fury whoami"`},
		{[]string{"logout", "now"}, `unknown command "now" for "fury logout"`},
		{[]string{"packages", "--json"}, "unknown flag: --json"},
	}

	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			server := offlineServer(t)

			term := terminal.NewForTest()
			cc := testContext(t, term, terminal.TestAuther("", "", nil), server)

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
	cc := cli.TestContext(t.Context(), terminal.NewForTest(), terminal.TestAuther("", "", nil))

	for _, cmd := range allCommands(cli.NewRootCommand(cc)) {
		if cmd.Runnable() && cmd.Args == nil {
			t.Errorf("Command %q has no Args check", cmd.CommandPath())
		}
	}
}

func TestUnknownCommandOutput(t *testing.T) {
	server := offlineServer(t)

	term := terminal.NewForTest()
	cc := testContext(t, term, terminal.TestAuther("", "", nil), server)
	expectExitStatus(t, term, runCommand(cc, []string{"nosuchcmd"}), cli.ExitUsage)
	expectOutput(t, term, "", "Error: unknown command \"nosuchcmd\" for \"fury\"\nRun 'fury --help' for usage.\n")
}

// expectInterrupted asserts that a command ended by being interrupted
func expectInterrupted(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}

// unconfirmed are the ways of not confirming a "y/N" question,
// and whether each interrupts the command rather than declines
var unconfirmed = map[string]bool{"ABORT": false, "INTERRUPT": true, "EOF": true}

// expectUnconfirmed asserts the outcome of a command whose confirmation was
// not given: either declined, which is no error, or left unanswered, which
// interrupts the command. Stdout is not checked, as it may hold the question.
func expectUnconfirmed(t *testing.T, term terminal.TestTerm, err error, interrupted bool) {
	t.Helper()
	if interrupted {
		expectInterrupted(t, err)
		expectErrOutput(t, term, "Cancelled\n")
	} else if err != nil {
		t.Errorf("Command error: %s", err)
	} else {
		expectErrOutput(t, term, "")
	}
}

// interruptOn handles a request by interrupting the command, by way of
// cancel, while that request is in flight. It counts the requests handled.
func interruptOn(cancel func(), requests *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		cancel()

		// The end of a request is noticed only once its body is read
		io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}
}

// An interrupted command stops at the item that it was on: the remaining
// items are not attempted, and only the interruption is reported, which
// is not about what the command asked for
func TestCommandsInterrupted(t *testing.T) {
	for name, tc := range map[string]struct {
		args   []string
		stdout string
	}{
		"versions":       {[]string{"versions", "foo"}, ""},
		"push":           {[]string{"push", samplePackagePath(), samplePackagePath()}, "Uploading sample.txt - cancelled\n"},
		"yank":           {[]string{"yank", "--force", "foo@1.0", "bar@1.0"}, ""},
		"sharing add":    {[]string{"sharing", "add", "a@example.com", "b@example.com"}, ""},
		"sharing remove": {[]string{"sharing", "remove", "a@example.com", "b@example.com"}, ""},
		"download":       {[]string{"beta", "download", "foo@1.0", "bar@1.0"}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			cc, cancel := context.WithCancel(cli.TestContext(t.Context(), term, auth))
			defer cancel()

			var requests atomic.Int32
			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/", interruptOn(cancel, &requests))
			})

			flags := ctx.GlobalFlags(cc)
			flags.PushEndpoint = server.URL
			flags.Endpoint = server.URL

			expectInterrupted(t, runCommand(cc, tc.args))
			if n := requests.Load(); n != 1 {
				t.Errorf("Expected 1 request before interruption, got %d", n)
			}

			expectOutput(t, term, tc.stdout, "Cancelled\n")
		})
	}
}

// An interruption between items leaves the command incomplete: it must
// fail, though no item has, and must not attempt the remaining items
func TestMultiItemCommandInterruptedBetweenItems(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	cc, cancel := context.WithCancel(cli.TestContext(t.Context(), term, auth))
	defer cancel()

	server := testutil.APIServer(t, "POST", "/uploads", pushResponse, 200)
	ctx.GlobalFlags(cc).PushEndpoint = server.URL

	// Interrupted once the first item is reported as done
	term.OnOutput(cancel)

	expectInterrupted(t, runCommand(cc, []string{"push", samplePackagePath(), samplePackagePath()}))
	expectOutput(t, term, "Uploading sample.txt - done\n", "Cancelled\n")
}

// mutableServer has what yank, git destroy, and logout would change,
// and counts the requests that do so
func mutableServer(t *testing.T, mutations *atomic.Int32) *httptest.Server {
	t.Helper()
	mutate := func(w http.ResponseWriter, r *http.Request) {
		mutations.Add(1)
		w.Write([]byte("{}"))
	}

	return testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /versions", func(w http.ResponseWriter, r *http.Request) {
			testutil.APIPaginatedResponse(t, w, r, versionsResponses, 200)
		})
		mux.HandleFunc("DELETE /packages/{pid}/versions/{vid}", mutate)
		mux.HandleFunc("DELETE /git/repos/me/repo-name", mutate)
		mux.HandleFunc("POST /logout", mutate)
	})
}

// With --yes, every "y/N" question is answered without being asked.
// No answers are given to the terminal, so a question would fail the command.
func TestConfirmationByFlag(t *testing.T) {
	for name, tc := range map[string]struct {
		args      []string
		noUser    bool // Without a terminal
		mutations int32
	}{
		"yank --yes":                 {[]string{"yank", "foo@0.0.1", "--yes"}, false, 2},
		"git destroy -y":             {[]string{"git", "destroy", "repo-name", "-y"}, false, 1},
		"logout --yes":               {[]string{"logout", "--yes"}, false, 1},
		"--yes before the command":   {[]string{"--yes", "logout"}, false, 1},
		"--yes without a terminal":   {[]string{"logout", "--yes"}, true, 1},
		"--force without a terminal": {[]string{"git", "destroy", "repo-name", "--force"}, true, 1},
		"--force --no-input":         {[]string{"yank", "foo@0.0.1", "--force", "--no-input"}, false, 2},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()
			term.SetInteractive(!tc.noUser)

			var mutations atomic.Int32
			server := mutableServer(t, &mutations)

			cc := testContext(t, term, auth, server)
			if err := runCommandNoErr(cc, tc.args); err != nil {
				t.Fatal(err)
			}

			if n := mutations.Load(); n != tc.mutations {
				t.Errorf("Expected %d requests to change things, got %d", tc.mutations, n)
			}
		})
	}
}

// With no one to ask, for lack of a terminal or by --no-input,
// a command that needs confirmation fails without changing anything
func TestConfirmationNeeded(t *testing.T) {
	for _, args := range [][]string{
		{"yank", "foo@0.0.1"},
		{"git", "destroy", "repo-name"},
		{"logout"},
	} {
		for name, flags := range map[string][]string{
			"without a terminal": nil,
			"with --no-input":    {"--no-input"},
		} {
			t.Run(strings.Join(args, " ")+" "+name, func(t *testing.T) {
				auth := terminal.TestAuther("user", "abc123", nil)
				term := terminal.NewForTest()
				term.SetInteractive(flags != nil) // At a terminal

				var mutations atomic.Int32
				server := mutableServer(t, &mutations)

				cc := testContext(t, term, auth, server)
				err := runCommand(cc, slices.Concat(args, flags))
				if !errors.Is(err, terminal.ErrNoInput) {
					t.Errorf("Expected terminal.ErrNoInput, got: %v", err)
				}

				expectErrOutput(t, term, "Error: Confirmation needed. Pass --yes to confirm, as there is no one to ask.\n")
				expectCredentials(t, auth, "user", "abc123")

				if n := mutations.Load(); n != 0 {
					t.Errorf("Expected no requests to change things, got %d", n)
				}
			})
		}
	}
}

// Context altering options added to test commands
type testOption func(context.Context) context.Context

// No login option
func noLoginOpt(cc context.Context) context.Context {
	ctx.Auther(cc).Wipe()
	return cc
}
