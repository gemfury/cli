package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// What the API responds with to the start of a browser login
const loginCreateResponse = `{"browser_url": "https://gemfury.com", "cli_url": "/cli/auth", "token": "xyz"}`

// Login is more or less the same as the "whoami" command
// because all commands force a login if logged out
// /login route is already present on APIServer

func TestLoginCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServer(t, "GET", "/users/me", whoamiResponse, 200)

	cc := cli.TestContext(t.Context(), term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	// Add any key for the "open browser" prompt
	term.InWrite([]byte("!"))

	err := runCommandNoErr(cc, []string{"login"})
	if err != nil {
		t.Error(err)
	}

	outStr := string(term.OutBytes())
	if exp := "You are logged in as \"u@example.com\"\n"; !strings.Contains(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

// Commands run with an Auther that tells login how the saving of the
// session went, or login would have nothing to say of it
func TestCommandContextAuther(t *testing.T) {
	if _, ok := ctx.Auther(cli.CommandContext()).(terminal.FallbackAuther); !ok {
		t.Error("Expected a terminal.FallbackAuther as the Auther of commands")
	}
}

// fallbackAuth saves the session as any Auther, and tells that it did so
// in the file at path, for want of a system keychain, or that Git is to
// be set by the commands of setup
type fallbackAuth struct {
	terminal.Auther
	path  string
	setup []string
}

func (a fallbackAuth) Fallback() string   { return a.path }
func (a fallbackAuth) GitSetup() []string { return a.setup }

// Login tells where the session is saved when that is not the system
// keychain, other than with --quiet. It warns of Git that could not be set
// to ask this CLI, with how to set it by hand.
func TestLoginCommandFallback(t *testing.T) {
	note := "Credentials are saved in /home/u/.netrc, as the system keychain is not available\n"
	warning := "Git is not set to ask this CLI for credentials, as its configuration could not be changed. To set it by hand:\n" +
		"  git config one\n  git config two\n"

	inNetrc := fallbackAuth{path: "/home/u/.netrc"}
	gitNotSet := fallbackAuth{setup: []string{"git config one", "git config two"}}

	for name, tc := range map[string]struct {
		auth   fallbackAuth
		args   []string
		notes  []string
		stderr string
	}{
		"in the keychain":         {fallbackAuth{}, []string{"login"}, nil, ""},
		"in .netrc":               {inNetrc, []string{"login"}, []string{note}, ""},
		"in .netrc, with --quiet": {inNetrc, []string{"login", "--quiet"}, nil, ""},
		"Git not set, --quiet":    {gitNotSet, []string{"login", "--quiet"}, nil, warning},
	} {
		t.Run(name, func(t *testing.T) {
			auth := tc.auth
			auth.Auther = terminal.TestAuther("", "", nil)
			term := terminal.NewForTest()

			server := testutil.APIServer(t, "GET", "/users/me", whoamiResponse, 200)
			cc := testContext(t, term, auth, server)

			// Add any key for the "open browser" prompt
			term.InWrite([]byte("!"))

			if err := runCommand(cc, tc.args); err != nil {
				t.Fatal(err)
			}

			expectCredentials(t, auth, "u@example.com", "token-abc-123")
			expectOutputLines(t, term, "the system keychain", tc.notes...)
			expectErrOutput(t, term, tc.stderr)
		})
	}
}

// errReadOnly is the failure of unsavingAuth and unwipingAuth
var errReadOnly = errors.New("read-only")

// unsavingAuth reads and wipes the saved session as any Auther,
// and fails to save one
type unsavingAuth struct {
	terminal.Auther
}

func (unsavingAuth) Append(string, string) error { return errReadOnly }

// unwipingAuth reads the saved session as any Auther, and fails to wipe it
type unwipingAuth struct {
	terminal.Auther
}

func (unwipingAuth) Wipe() error { return errReadOnly }

// revoking records the token of each revocation, which succeeds
func revoking(revoked *[]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*revoked = append(*revoked, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusNoContent)
	}
}

// A token that cannot be saved is revoked, rather than left valid
// where no one has it
func TestLoginCommandNotSaved(t *testing.T) {
	auth := unsavingAuth{terminal.TestAuther("", "", nil)}
	term := terminal.NewForTest()

	var revoked []string
	server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
		h.HandleFunc("/logout", revoking(&revoked))
	})

	// Add any key for the "open browser" prompt
	term.InWrite([]byte("!"))

	cc := testContext(t, term, auth, server)
	if err := runCommand(cc, []string{"login"}); !errors.Is(err, errReadOnly) {
		t.Errorf("Expected the failure to save, got: %v", err)
	}

	if exp := []string{"token-abc-123"}; !slices.Equal(revoked, exp) {
		t.Errorf("Expected revoked tokens %q, got %q", exp, revoked)
	}
}

// A session that is revoked, and cannot be wiped, fails the logout,
// as it does the login that would replace it
func TestSessionNotWiped(t *testing.T) {
	for _, args := range [][]string{{"logout", "--yes"}, {"login"}} {
		t.Run(args[0], func(t *testing.T) {
			auth := unwipingAuth{terminal.TestAuther("user", "abc123", nil)}
			term := terminal.NewForTest()

			server := testutil.APIServer(t, "POST", "/logout", "{}", 200)
			cc := testContext(t, term, auth, server)

			if err := runCommand(cc, args); !errors.Is(err, errReadOnly) {
				t.Errorf("Expected the failure to wipe, got: %v", err)
			}

			expectOutput(t, term, "", "Error: read-only\n")
		})
	}
}

func TestLoginCommandInteractive(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServer(t, "GET", "/users/me", whoamiResponse, 200)

	cc := cli.TestContext(t.Context(), term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	term.SetPromptResponses(map[string]string{
		"Email: ":    "u@example.com",
		"Password: ": "secreto",
	})

	err := runCommandNoErr(cc, []string{"login", "--interactive"})
	if err != nil {
		t.Error(err)
	}

	outStr := string(term.OutBytes())
	if exp := "You are logged in as \"u@example.com\"\n"; !strings.Contains(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

func TestLoginCommandInteractiveFallback(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Fire up test server that returns 501 for /cli/auth
	server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
		h.HandleFunc("/cli/auth", respondWith(http.StatusNotImplemented, ""))
		h.HandleFunc("/users/me", respondWith(200, whoamiResponse))
	})

	cc := cli.TestContext(t.Context(), term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	term.SetPromptResponses(map[string]string{
		"Email: ":    "u@example.com",
		"Password: ": "secreto",
	})

	// User requests browser, but we fall back to interactive
	err := runCommandNoErr(cc, []string{"login"})
	if err != nil {
		t.Error(err)
	}

	outStr := string(term.OutBytes())
	if exp := "You are logged in as \"u@example.com\"\n"; !strings.Contains(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}
}

// Browser prompt cancelled with Ctrl-C: "login" exits quietly,
// and the browser is never opened nor polled for a token
func TestLoginCommandCancelled(t *testing.T) {
	for _, key := range []byte{'q', 0x03, 0x04, 0x1b} {
		t.Run(fmt.Sprintf("key %#x", key), func(t *testing.T) {
			auth := terminal.TestAuther("", "", nil)
			term := terminal.NewForTest()

			server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
				h.HandleFunc("/cli/auth", func(w http.ResponseWriter, r *http.Request) {
					if r.Method != "POST" {
						t.Errorf("Login should not be polled after cancel")
					}
					w.Write([]byte(loginCreateResponse))
				})
			})

			cc := testContext(t, term, auth, server)
			term.InWrite([]byte{key})
			if err := runCommandNoErr(cc, []string{"login"}); err != nil {
				t.Error(err)
			}

			if outStr := string(term.OutBytes()); strings.Contains(outStr, "Opening") {
				t.Errorf("Browser should not be opened, got %q", outStr)
			}

			expectCredentials(t, auth, "", "")
		})
	}
}

// Other commands report the cancellation as a readable error
func TestCommandLoginCancelled(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Only the default login handlers; reaching /packages fails the test
	server := testutil.APIServerCustom(t, func(*http.ServeMux) {})

	cc := testContext(t, term, auth, server)
	term.InWrite([]byte{0x03})
	err := runCommand(cc, []string{"packages"})
	if err == nil || err.Error() != "Login cancelled" {
		t.Fatalf("Expected 'Login cancelled' error, got: %v", err)
	}

	expectErrOutput(t, term, "Error: Login cancelled\n")
}

// Without credentials and without a terminal (pipe, CI), commands fail
// fast: no login is started, nothing is requested, prompted, or saved.
// Likewise at a terminal, when its user is not to be asked.
func TestCommandNotLoggedInNonInteractive(t *testing.T) {
	for _, args := range [][]string{
		{"packages"},
		{"whoami"},
		{"packages", "--no-input"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			auth := terminal.TestAuther("", "", nil)
			term := terminal.NewForTest()
			term.SetInteractive(slices.Contains(args, "--no-input")) // At a terminal

			server := offlineServer(t)

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, args)
			if !errors.Is(err, cli.ErrNotLoggedIn) {
				t.Fatalf("Expected cli.ErrNotLoggedIn, got: %v", err)
			}

			expectOutput(t, term, "", "Error: Not logged in. Set FURY_TOKEN or run \"fury login\" in a terminal.\n")

			expectCredentials(t, auth, "", "")
		})
	}
}

// Without a terminal, or a user to be asked at one, there is no way to
// login: the saved session is kept, rather than revoked for nothing
func TestLoginCommandNonInteractive(t *testing.T) {
	for _, args := range [][]string{
		{"login"},
		{"login", "--interactive"},
		{"login", "--yes"},
		{"login", "--no-input"},
	} {
		for name, saved := range map[string]string{"logged out": "", "logged in": "abc123"} {
			t.Run(strings.Join(args, " ")+", "+name, func(t *testing.T) {
				auth := terminal.TestAuther("", saved, nil)
				term := terminal.NewForTest()
				term.SetInteractive(slices.Contains(args, "--no-input")) // At a terminal

				// Any request would be to revoke the session, or to login
				server := offlineServer(t)

				cc := testContext(t, term, auth, server)
				err := runCommand(cc, args)
				if !errors.Is(err, cli.ErrLoginUnattended) {
					t.Fatalf("Expected cli.ErrLoginUnattended, got: %v", err)
				}

				expectOutput(t, term, "", "Error: Cannot login with no one to ask. Pass --api-token to save a session instead.\n")
				expectCredentials(t, auth, "", saved)
			})
		}
	}
}

// Logging in over a saved session revokes the saved token, not any other
func TestLoginCommandReplacesSavedToken(t *testing.T) {
	auth := terminal.TestAuther("old@example.com", "old-token", nil)
	term := terminal.NewForTest()

	var revoked []string
	server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
		h.HandleFunc("/logout", revoking(&revoked))
		h.HandleFunc("/users/me", respondWith(200, whoamiResponse))
	})

	cc := testContext(t, term, auth, server)
	term.InWrite([]byte("!"))
	if err := runCommandNoErr(cc, []string{"login"}); err != nil {
		t.Fatal(err)
	}

	if exp := []string{"old-token"}; !slices.Equal(revoked, exp) {
		t.Errorf("Expected revoked tokens %q, got %q", exp, revoked)
	}

	expectCredentials(t, auth, "u@example.com", "token-abc-123")
}

// With a token from the environment, login only verifies that token;
// saved credentials are neither revoked nor replaced
func TestLoginCommandVerifiesEnvToken(t *testing.T) {
	t.Setenv("FURY_TOKEN", "env-token")
	auth := terminal.TestAuther("old@example.com", "old-token", nil)
	term := terminal.NewForTest()

	// Only /users/me: a revoke would be an unexpected request, failing the test
	server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
		h.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
			if a := r.Header.Get("Authorization"); a != "env-token" {
				t.Errorf("Expected the environment token to be verified, got %q", a)
			}
			w.Write([]byte(whoamiResponse))
		})
	})

	cc := testContext(t, term, auth, server)
	if err := runCommandNoErr(cc, []string{"login"}); err != nil {
		t.Fatal(err)
	}

	expectOutput(t, term, "API token belongs to \"joetest\"\n", "")
	expectCredentials(t, auth, "old@example.com", "old-token")
}

// With a token by the flag, given or read from stdin, login verifies it,
// then saves it as the session. The one saved before is revoked, unless
// it is the same token
func TestLoginCommandSavesTokenFlag(t *testing.T) {
	for name, tc := range map[string]struct {
		token   string            // Given to --api-token
		stdin   string            // Piped, when there is no user
		prompt  map[string]string // Asked, when there is
		saved   string            // The token saved before
		revoked []string
	}{
		"flag":         {"new-token", "", nil, "old-token", []string{"old-token"}},
		"stdin piped":  {"-", "new-token\n", nil, "old-token", []string{"old-token"}},
		"stdin prompt": {"-", "", map[string]string{"API token: ": "new-token"}, "old-token", []string{"old-token"}},
		"same token":   {"new-token", "", nil, "new-token", nil},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("old@example.com", tc.saved, nil)
			term := terminal.NewForTest()
			term.SetInteractive(tc.prompt != nil)
			term.InWrite([]byte(tc.stdin))
			term.SetPromptResponses(tc.prompt)

			var revoked []string
			server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
				h.HandleFunc("/logout", revoking(&revoked))
				h.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
					if a := r.Header.Get("Authorization"); a != "new-token" {
						t.Errorf("Expected the new token to be verified, got %q", a)
					}
					w.Write([]byte(whoamiResponse))
				})
			})

			cc := testContext(t, term, auth, server)
			if err := runCommandNoErr(cc, []string{"login", "--api-token", tc.token}); err != nil {
				t.Fatal(err)
			}

			expectOutput(t, term, "You are logged in as \"joe@example.com\"\n", "")
			expectCredentials(t, auth, "joe@example.com", "new-token")
			if !slices.Equal(revoked, tc.revoked) {
				t.Errorf("Expected revoked tokens %q, got %q", tc.revoked, revoked)
			}
		})
	}
}

// A token of an organization, which has no email, is saved by its username
func TestLoginCommandSavesOrgToken(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	const orgResponse = `{"name": "test-org", "username": "test-org"}`
	server := testutil.APIServer(t, "GET", "/users/me", orgResponse, 200)

	cc := testContext(t, term, auth, server)
	if err := runCommandNoErr(cc, []string{"login", "--api-token", "org-token"}); err != nil {
		t.Fatal(err)
	}

	expectOutput(t, term, "You are logged in as \"test-org\"\n", "")
	expectCredentials(t, auth, "test-org", "org-token")
}

// A token that is refused replaces nothing: the saved session is kept
func TestLoginCommandTokenRefused(t *testing.T) {
	t.Setenv("FURY_TOKEN", "bad-token")

	for name, args := range map[string][]string{
		"flag": {"login", "--api-token", "bad-token"},
		"env":  {"login"},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("old@example.com", "old-token", nil)
			term := terminal.NewForTest()

			server := testutil.APIServer(t, "GET", "/users/me", `{"error": "Token has expired"}`, 401)

			cc := testContext(t, term, auth, server)
			expectExitStatus(t, term, runCommand(cc, args), cli.ExitAuth)
			expectOutput(t, term, "", "Error: Token has expired\n")
			expectCredentials(t, auth, "old@example.com", "old-token")
		})
	}
}

// When the server refuses to revoke the saved session, the user is asked
// whether to go on without it, as at a browser login; with no one to ask,
// the login fails. The new token is verified by then, and saved only
// once that is settled.
func TestLoginCommandTokenRevokeFails(t *testing.T) {
	const confirm = "Do you want to ignore & continue with your login? [y/N]"

	for name, tc := range map[string]struct {
		flags  []string
		noUser bool
		answer string // To confirm, when asked
		cause  error  // Of the failure, or nil when saved
	}{
		"--yes":       {[]string{"--yes"}, false, "", nil},
		"confirmed":   {nil, false, "y", nil},
		"declined":    {nil, false, "ABORT", api.ErrFuryServer},
		"interrupted": {nil, false, "INTERRUPT", context.Canceled},
		"piped":       {nil, true, "", terminal.ErrNoInput},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("old@example.com", "old-token", nil)
			term := terminal.NewForTest()
			term.SetInteractive(!tc.noUser)
			term.SetPromptResponses(map[string]string{confirm: tc.answer})

			server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
				h.HandleFunc("/logout", respondWith(500, ""))
				h.HandleFunc("/users/me", respondWith(200, whoamiResponse))
			})

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, slices.Concat([]string{"login", "--api-token", "new-token"}, tc.flags))
			if tc.cause == nil {
				if err != nil {
					t.Fatal(err)
				}
				expectCredentials(t, auth, "joe@example.com", "new-token")
			} else {
				if !errors.Is(err, tc.cause) {
					t.Errorf("Expected %v within error, got: %v", tc.cause, err)
				}
				expectCredentials(t, auth, "old@example.com", "old-token")
			}

			expectProblems(t, term, "Error deactivating your old CLI credentials: ")
		})
	}
}

// A given token that cannot be saved is not revoked, unlike one minted by
// a browser login, as it is the user's to keep. The session saved before
// is gone by then.
func TestLoginCommandTokenNotSaved(t *testing.T) {
	for name, tc := range map[string]struct {
		saved   string
		revoked []string
	}{
		"old session": {"old-token", []string{"old-token"}},
		"no session":  {"", nil},
	} {
		t.Run(name, func(t *testing.T) {
			auth := unsavingAuth{terminal.TestAuther("", tc.saved, nil)}
			term := terminal.NewForTest()

			var revoked []string
			server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
				h.HandleFunc("/logout", revoking(&revoked))
				h.HandleFunc("/users/me", respondWith(200, whoamiResponse))
			})

			cc := testContext(t, term, auth, server)
			if err := runCommand(cc, []string{"login", "--api-token", "new-token"}); !errors.Is(err, errReadOnly) {
				t.Errorf("Expected the failure to save, got: %v", err)
			}

			if !slices.Equal(revoked, tc.revoked) {
				t.Errorf("Expected revoked tokens %q, got %q", tc.revoked, revoked)
			}
			expectCredentials(t, auth, "", "")
		})
	}
}

// Logout acts on the saved credentials, whatever FURY_TOKEN holds
func TestLogoutCommandSuccess(t *testing.T) {
	for name, env := range map[string]string{"without env": "", "with env": "env-token"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FURY_TOKEN", env)
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			// Fire up test server; the saved token must be the one revoked
			var revoked []string
			server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
				h.HandleFunc("/logout", revoking(&revoked))
			})

			term.SetPromptResponses(map[string]string{
				"Are you sure you want to logout? [y/N]": "Y",
			})

			cc := testContext(t, term, auth, server)
			if err := runCommandNoErr(cc, []string{"logout"}); err != nil {
				t.Error(err)
			}

			expectOutput(t, term, "You have been logged out\n", "")

			if exp := []string{"abc123"}; !slices.Equal(revoked, exp) {
				t.Errorf("Expected revoked tokens %q, got %q", exp, revoked)
			}

			expectCredentials(t, auth, "", "")
		})
	}
}

// When the server refuses to revoke the saved token, the failure is reported
// on stderr and the user decides whether to wipe the saved credentials anyway.
// Leaving that undecided interrupts the command.
func TestLogoutCommandRevokeFails(t *testing.T) {
	for _, tc := range []struct {
		answer      string
		wiped       bool // Otherwise the command fails
		interrupted bool
	}{
		{answer: "Y", wiped: true},
		{answer: "ABORT"},
		{answer: "INTERRUPT", interrupted: true},
		{answer: "EOF", interrupted: true},
	} {
		t.Run("wipe anyway: "+tc.answer, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			server := testutil.APIServer(t, "POST", "/logout", "", 500)

			term.SetPromptResponses(map[string]string{
				"Are you sure you want to logout? [y/N]":                "Y",
				"Do you want to remove saved credentials anyway? [y/N]": tc.answer,
			})

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, []string{"logout"})
			if (err == nil) != tc.wiped {
				t.Errorf("Expected wiped=%v, got error: %v", tc.wiped, err)
			} else if errors.Is(err, context.Canceled) != tc.interrupted {
				t.Errorf("Expected interrupted=%v, got: %v", tc.interrupted, err)
			}

			expectProblems(t, term, "Error deactivating your old CLI credentials: ")

			if wiped := auth.User == "" && auth.Pass == ""; wiped != tc.wiped {
				t.Errorf("Expected wiped=%v, got auth %+v", tc.wiped, auth)
			}
		})
	}
}

// Interrupted while revoking the saved token, which is not a refusal
// to revoke it: nothing is asked, and the credentials are retained
func TestLogoutCommandInterrupted(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	cc, cancel := context.WithCancel(cli.TestContext(t.Context(), term, auth))
	defer cancel()

	var requests atomic.Int32
	server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
		h.HandleFunc("/logout", interruptOn(cancel, &requests))
	})

	ctx.GlobalFlags(cc).Endpoint = server.URL
	term.SetPromptResponses(map[string]string{
		"Are you sure you want to logout? [y/N]":                "Y",
		"Do you want to remove saved credentials anyway? [y/N]": "Y", // Never asked
	})

	expectInterrupted(t, runCommand(cc, []string{"logout"}))
	if n := requests.Load(); n != 1 {
		t.Errorf("Expected 1 revocation before interruption, got %d", n)
	}

	expectOutput(t, term, "", "Cancelled\n")

	expectCredentials(t, auth, "user", "abc123")
}

// Without saved credentials, there is nothing to ask or to revoke
func TestLogoutCommandLoggedOut(t *testing.T) {
	for command, stdout := range map[string]string{"logout": "You are logged out\n", "logout --quiet": ""} {
		t.Run(command, func(t *testing.T) {
			auth := terminal.TestAuther("", "", nil)
			term := terminal.NewForTest()

			cc := testContext(t, term, auth, offlineServer(t))
			if err := runCommand(cc, strings.Fields(command)); err != nil {
				t.Fatal(err)
			}

			expectOutput(t, term, stdout, "")
		})
	}
}

// Logout rejects --api-token as a usage error, without reading stdin
// for "-": nothing is revoked or wiped
func TestLogoutCommandWithTokenFlag(t *testing.T) {
	for _, token := range []string{"other", "-"} {
		t.Run(token, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			// Any request would be a wrongful revocation
			server := offlineServer(t)

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, []string{"logout", "--api-token", token})
			if !cli.IsUsageError(err) {
				t.Fatalf("Expected usage error, got: %v", err)
			}

			expectProblems(t, term, "Error: Logout clears saved credentials only; do not pass --api-token\n")
			expectCredentials(t, auth, "user", "abc123")
		})
	}
}

// Credentials are retained unless confirmed
func TestLogoutCommandUnconfirmed(t *testing.T) {
	for answer, interrupted := range unconfirmed {
		t.Run(answer, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			// Any request would be a wrongful revocation
			server := offlineServer(t)

			term.SetPromptResponses(map[string]string{
				"Are you sure you want to logout? [y/N]": answer,
			})

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, []string{"logout"})
			expectUnconfirmed(t, term, err, interrupted)

			if out := string(term.OutBytes()); out != "" {
				t.Errorf("Expected no output, got %q", out)
			}

			expectCredentials(t, auth, "user", "abc123")
		})
	}
}

// Backing out of the email or password prompt, in any way, cancels the login
func TestInteractiveLoginCancelled(t *testing.T) {
	for name, prompts := range map[string]map[string]string{
		"email ABORT":        {"Email: ": "ABORT"},
		"email INTERRUPT":    {"Email: ": "INTERRUPT"},
		"email EOF":          {"Email: ": "EOF"},
		"password INTERRUPT": {"Email: ": "u@example.com", "Password: ": "INTERRUPT"},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("", "", nil)

			// Interactive login is the fallback of a browser login
			server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
				h.HandleFunc("/cli/auth", respondWith(http.StatusNotImplemented, ""))
			})

			// "login" exits quietly
			term := terminal.NewForTest()
			term.SetPromptResponses(prompts)
			cc := testContext(t, term, auth, server)
			if err := runCommandNoErr(cc, []string{"login", "--interactive"}); err != nil {
				t.Error(err)
			}

			// Other commands report the cancellation
			term = terminal.NewForTest()
			term.SetPromptResponses(prompts)
			cc = testContext(t, term, auth, server)
			err := runCommand(cc, []string{"packages"})
			if err == nil || err.Error() != "Login cancelled" {
				t.Errorf("Expected 'Login cancelled' error, got: %v", err)
			}

			expectErrOutput(t, term, "Error: Login cancelled\n")

			expectCredentials(t, auth, "", "")
		})
	}
}

// pendingLoginServer never approves the browser login: polling blocks
// until the CLI gives up. onPoll is called as each poll arrives.
func pendingLoginServer(t *testing.T, onPoll func()) *httptest.Server {
	t.Helper()
	return testutil.APIServerCustom(t, func(h *http.ServeMux) {
		h.HandleFunc("POST /cli/auth", respondWith(200, loginCreateResponse))
		h.HandleFunc("GET /cli/auth", stall(onPoll))
	})
}

// Interrupting the wait for a browser login ends the command right away
func TestLoginCommandInterrupted(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	cc, cancel := context.WithCancel(cli.TestContext(t.Context(), term, auth))
	defer cancel()

	server := pendingLoginServer(t, cancel)

	ctx.GlobalFlags(cc).Endpoint = server.URL
	term.InWrite([]byte("!"))

	start := time.Now()
	err := runCommand(cc, []string{"login"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got: %v", err)
	} else if d := time.Since(start); d > 5*time.Second {
		t.Errorf("Expected a prompt exit, took %s", d)
	}

	expectErrOutput(t, term, "Cancelled\n")

	expectCredentials(t, auth, "", "")
}

// Interrupted while opening the browser, which is not a failure to open it
func TestLoginCommandInterruptedOpeningBrowser(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	cc, cancel := context.WithCancel(cli.TestContext(t.Context(), term, auth))
	defer cancel()

	server := pendingLoginServer(t, func() {
		t.Errorf("Login should not be polled after interruption")
	})

	ctx.GlobalFlags(cc).Endpoint = server.URL
	term.InWrite([]byte("!"))

	// Interrupted at the prompt, which is before the browser is opened
	term.OnOutput(cancel)

	expectInterrupted(t, runCommand(cc, []string{"login"}))
	if out := string(term.OutBytes()); !strings.Contains(out, "Opening ") || strings.Contains(out, "Failed to open") {
		t.Errorf("Expected the browser to be opening, not failing, got %q", out)
	}

	expectErrOutput(t, term, "Cancelled\n")
}

// A browser login is asked about for as long as it is pending, which the
// API tells by a 404 or a 408, in the body of its response or not. Any
// other refusal ends the login at once.
func TestLoginCommandPolling(t *testing.T) {
	for name, tc := range map[string]struct {
		polls  []int // Status of each response, in order
		exp    int
		stderr string
	}{
		"pending, then approved": {[]int{404, 408, 200}, cli.ExitOK, ""},
		"refused":                {[]int{500}, cli.ExitUnavailable, "Error: Try later (HTTP 500)\n"},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("", "", nil)
			term := terminal.NewForTest()

			var polls atomic.Int32
			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/users/me", respondWith(200, whoamiResponse))
				mux.HandleFunc("GET /cli/auth", func(w http.ResponseWriter, r *http.Request) {
					switch status := tc.polls[polls.Add(1)-1]; status {
					case 200:
						respondWith(200, `{"user": {"email": "u@example.com"}, "token": "token-abc-123"}`)(w, r)
					case 408:
						respondWith(408, "")(w, r)
					default:
						respondWith(status, `{"error": "Try later"}`)(w, r)
					}
				})
				mux.HandleFunc("POST /cli/auth", respondWith(200, loginCreateResponse))
			})

			cli.SetLoginPollInterval(t, time.Millisecond)
			cc := testContext(t, term, auth, server)
			term.InWrite([]byte("!"))

			expectExitStatus(t, term, runCommand(cc, []string{"login"}), tc.exp)
			expectErrOutput(t, term, tc.stderr)

			if n := int(polls.Load()); n != len(tc.polls) {
				t.Errorf("Expected %d polls, got %d", len(tc.polls), n)
			}
		})
	}
}

// A browser login that is never approved times out, poll in flight or not
func TestLoginCommandTimeout(t *testing.T) {
	t.Setenv("FURY_RETRIES", "3") // Retries are on: a poll cut short by the timeout is not sent again
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	var polls atomic.Int32
	server := pendingLoginServer(t, func() { polls.Add(1) })

	cli.SetLoginPollTimeout(t, 100*time.Millisecond)
	cc := testContext(t, term, auth, server)
	term.InWrite([]byte("!"))

	err := runCommand(cc, []string{"login"})
	if !errors.Is(err, api.ErrTimeout) {
		t.Errorf("Expected api.ErrTimeout, got: %v", err)
	}

	expectErrOutput(t, term, "Error: Operation timed out. Try again later.\n")
	if n := polls.Load(); n != 1 {
		t.Errorf("Expected 1 poll, got %d", n)
	}
}
