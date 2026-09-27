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
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

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
		h.HandleFunc("/cli/auth", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})
		h.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(whoamiResponse))
		})
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
					w.Write([]byte(`{"browser_url": "https://gemfury.com", "cli_url": "/cli/auth", "token": "xyz"}`))
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
// fast: no login is started, nothing is requested, prompted, or saved
func TestCommandNotLoggedInNonInteractive(t *testing.T) {
	for _, args := range [][]string{{"packages"}, {"whoami"}, {"login"}, {"login", "--interactive"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			auth := terminal.TestAuther("", "", nil)
			term := terminal.NewForTest()
			term.SetInteractive(false)

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

// Logging in over a saved session revokes the saved token, not any other
func TestLoginCommandReplacesSavedToken(t *testing.T) {
	auth := terminal.TestAuther("old@example.com", "old-token", nil)
	term := terminal.NewForTest()

	var revoked string
	server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
		h.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
			revoked = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusNoContent)
		})
		h.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(whoamiResponse))
		})
	})

	cc := testContext(t, term, auth, server)
	term.InWrite([]byte("!"))
	if err := runCommandNoErr(cc, []string{"login"}); err != nil {
		t.Fatal(err)
	}

	if exp := "old-token"; revoked != exp {
		t.Errorf("Expected %q to be revoked, got %q", exp, revoked)
	}

	expectCredentials(t, auth, "u@example.com", "token-abc-123")
}

// With an inline token, login only verifies that token;
// saved credentials are neither revoked nor replaced
func TestLoginCommandWithInlineTokenKeepsSaved(t *testing.T) {
	for name, tc := range map[string]struct {
		env  string
		args []string
	}{
		"flag": {"", []string{"login", "--api-token", "inline-token"}},
		"env":  {"inline-token", []string{"login"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FURY_TOKEN", tc.env)
			auth := terminal.TestAuther("old@example.com", "old-token", nil)
			term := terminal.NewForTest()

			server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
				h.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
					t.Errorf("Unexpected revoke of %q", r.Header.Get("Authorization"))
					w.WriteHeader(http.StatusNoContent)
				})
				h.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
					if a := r.Header.Get("Authorization"); a != "inline-token" {
						t.Errorf("Expected inline token to be verified, got %q", a)
					}
					w.Write([]byte(whoamiResponse))
				})
			})

			cc := testContext(t, term, auth, server)
			if err := runCommandNoErr(cc, tc.args); err != nil {
				t.Fatal(err)
			}

			expectOutput(t, term, "API token belongs to \"joetest\"\n", "")

			expectCredentials(t, auth, "old@example.com", "old-token")
		})
	}
}

func TestLoginCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/users/me", whoamiResponse, 200)
	testCommandLoginPreCheck(t, []string{"login"}, server, noLoginOpt)
	server.Close()
}

// Logout acts on the saved credentials, whatever FURY_TOKEN holds
func TestLogoutCommandSuccess(t *testing.T) {
	for name, env := range map[string]string{"without env": "", "with env": "env-token"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FURY_TOKEN", env)
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			// Fire up test server; the saved token must be the one revoked
			var revoked int
			server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
				h.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
					if a := r.Header.Get("Authorization"); a != "abc123" {
						t.Errorf("Expected saved token to be revoked, got %q", a)
					}
					revoked++
					w.WriteHeader(http.StatusNoContent)
				})
			})

			term.SetPromptResponses(map[string]string{
				"Are you sure you want to logout? [y/N]": "Y",
			})

			cc := testContext(t, term, auth, server)
			if err := runCommandNoErr(cc, []string{"logout"}); err != nil {
				t.Error(err)
			}

			expectOutput(t, term, "You have been logged out\n", "")

			if revoked != 1 {
				t.Errorf("Expected one revocation, got %d", revoked)
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
				"Are you sure you want to logout? [y/N]":                      "Y",
				"Do you want to remove credentials from .netrc anyway? [y/N]": tc.answer,
			})

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, []string{"logout"})
			if (err == nil) != tc.wiped {
				t.Errorf("Expected wiped=%v, got error: %v", tc.wiped, err)
			} else if errors.Is(err, context.Canceled) != tc.interrupted {
				t.Errorf("Expected interrupted=%v, got: %v", tc.interrupted, err)
			}

			errStr := string(term.ErrBytes())
			if exp := "Error deactivating your old CLI credentials: "; !strings.HasPrefix(errStr, exp) {
				t.Errorf("Expected revoke failure on stderr, got %q", errStr)
			}

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
		"Are you sure you want to logout? [y/N]":                      "Y",
		"Do you want to remove credentials from .netrc anyway? [y/N]": "Y", // Never asked
	})

	expectInterrupted(t, runCommand(cc, []string{"logout"}))
	if n := requests.Load(); n != 1 {
		t.Errorf("Expected 1 revocation before interruption, got %d", n)
	}

	expectOutput(t, term, "", "Cancelled\n")

	expectCredentials(t, auth, "user", "abc123")
}

// Logout with --api-token is a usage error: nothing is revoked or wiped
func TestLogoutCommandWithTokenFlag(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Any request would be a wrongful revocation
	server := offlineServer(t)

	cc := testContext(t, term, auth, server)
	err := runCommand(cc, []string{"logout", "--api-token", "other"})
	if !cli.IsUsageError(err) {
		t.Fatalf("Expected usage error, got: %v", err)
	}

	expectCredentials(t, auth, "user", "abc123")
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
				h.HandleFunc("/cli/auth", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotImplemented)
				})
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
		h.HandleFunc("/cli/auth", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" {
				w.Write([]byte(`{"browser_url": "https://gemfury.com", "cli_url": "/cli/auth", "token": "xyz"}`))
				return
			}
			onPoll()
			<-r.Context().Done()
		})
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

// A browser login that is never approved times out, poll in flight or not
func TestLoginCommandTimeout(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	server := pendingLoginServer(t, func() {})

	cli.SetLoginPollTimeout(t, 100*time.Millisecond)
	cc := testContext(t, term, auth, server)
	term.InWrite([]byte("!"))

	err := runCommand(cc, []string{"login"})
	if !errors.Is(err, api.ErrTimeout) {
		t.Errorf("Expected api.ErrTimeout, got: %v", err)
	}

	expectErrOutput(t, term, "Error: Operation timed out. Try again later.\n")
}
