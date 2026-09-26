package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Login is more or less the same as the "whoami" command
// because all commands force a login if logged out
// /login route is already present on APIServer

func TestLoginCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Fire up test server
	server := testutil.APIServer(t, "GET", "/users/me", whoamiResponse, 200)
	defer server.Close()

	cc := cli.TestContext(term, auth)
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
	defer server.Close()

	cc := cli.TestContext(term, auth)
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
	defer server.Close()

	cc := cli.TestContext(term, auth)
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
			defer server.Close()

			cc := testContext(term, auth, server)
			term.InWrite([]byte{key})
			if err := runCommandNoErr(cc, []string{"login"}); err != nil {
				t.Error(err)
			}

			if outStr := string(term.OutBytes()); strings.Contains(outStr, "Opening") {
				t.Errorf("Browser should not be opened, got %q", outStr)
			}

			if u, p, _ := auth.Auth(); u != "" || p != "" {
				t.Errorf("Expected no saved credentials, got %q/%q", u, p)
			}
		})
	}
}

// Other commands report the cancellation as a readable error
func TestCommandLoginCancelled(t *testing.T) {
	auth := terminal.TestAuther("", "", nil)
	term := terminal.NewForTest()

	// Only the default login handlers; reaching /packages fails the test
	server := testutil.APIServerCustom(t, func(*http.ServeMux) {})
	defer server.Close()

	cc := testContext(term, auth, server)
	term.InWrite([]byte{0x03})
	err := runCommand(cc, []string{"packages"})
	if err == nil || err.Error() != "Login cancelled" {
		t.Fatalf("Expected 'Login cancelled' error, got: %v", err)
	}

	if exp := "Error: Login cancelled\n"; string(term.ErrBytes()) != exp {
		t.Errorf("Expected %q on stderr, got %q", exp, term.ErrBytes())
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
	defer server.Close()

	cc := testContext(term, auth, server)
	term.InWrite([]byte("!"))
	if err := runCommandNoErr(cc, []string{"login"}); err != nil {
		t.Fatal(err)
	}

	if exp := "old-token"; revoked != exp {
		t.Errorf("Expected %q to be revoked, got %q", exp, revoked)
	}

	if u, p, _ := auth.Auth(); u != "u@example.com" || p != "token-abc-123" {
		t.Errorf("Expected new credentials saved, got %q/%q", u, p)
	}
}

// With --api-token, login only verifies the token; saved credentials
// are neither revoked nor replaced
func TestLoginCommandWithTokenFlagKeepsSaved(t *testing.T) {
	auth := terminal.TestAuther("old@example.com", "old-token", nil)
	term := terminal.NewForTest()

	server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
		h.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("Unexpected revoke of %q", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusNoContent)
		})
		h.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
			if a := r.Header.Get("Authorization"); a != "flag-token" {
				t.Errorf("Expected flag token to be verified, got %q", a)
			}
			w.Write([]byte(whoamiResponse))
		})
	})
	defer server.Close()

	cc := testContext(term, auth, server)
	if err := runCommandNoErr(cc, []string{"login", "--api-token", "flag-token"}); err != nil {
		t.Fatal(err)
	}

	if exp := "API token belongs to \"joetest\"\n"; string(term.OutBytes()) != exp {
		t.Errorf("Expected output %q, got %q", exp, term.OutBytes())
	}

	if u, p, _ := auth.Auth(); u != "old@example.com" || p != "old-token" {
		t.Errorf("Expected saved credentials untouched, got %q/%q", u, p)
	}
}

func TestLoginCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/users/me", whoamiResponse, 200)
	testCommandLoginPreCheck(t, []string{"login"}, server, noLoginOpt)
	server.Close()
}

// Login is more or less the same as the "whoami" command
// because all commands force a login if logged out

func TestLogoutCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server; the saved token must be the one revoked
	server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
		h.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
			if a := r.Header.Get("Authorization"); a != "abc123" {
				t.Errorf("Expected saved token to be revoked, got %q", a)
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})
	defer server.Close()

	term.SetPromptResponses(map[string]string{
		"Are you sure you want to logout? [y/N]": "Y",
	})

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"logout"})
	if err != nil {
		t.Error(err)
	}

	outStr := string(term.OutBytes())
	if exp := "You have been logged out\n"; outStr != exp {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	if auth.User != "" || auth.Pass != "" || auth.Err != nil {
		t.Errorf("Expected command to wipe auth: %+v", auth)
	}
}

// When the server refuses to revoke the saved token, the failure is reported
// on stderr and the user decides whether to wipe the saved credentials anyway
func TestLogoutCommandRevokeFails(t *testing.T) {
	for _, tc := range []struct {
		answer   string
		wiped    bool
		finalErr bool
	}{
		{answer: "Y", wiped: true, finalErr: false},
		{answer: "ABORT", wiped: false, finalErr: true},
	} {
		t.Run("wipe anyway: "+tc.answer, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			server := testutil.APIServer(t, "POST", "/logout", "", 500)
			defer server.Close()

			term.SetPromptResponses(map[string]string{
				"Are you sure you want to logout? [y/N]":                      "Y",
				"Do you want to remove credentials from .netrc anyway? [y/N]": tc.answer,
			})

			cc := testContext(term, auth, server)
			err := runCommand(cc, []string{"logout"})
			if (err != nil) != tc.finalErr {
				t.Errorf("Expected error=%v, got: %v", tc.finalErr, err)
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

// Logout with --api-token is a usage error: nothing is revoked or wiped
func TestLogoutCommandWithTokenFlag(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Any request would be a wrongful revocation
	server := offlineServer(t)
	defer server.Close()

	cc := testContext(term, auth, server)
	err := runCommand(cc, []string{"logout", "--api-token", "other"})
	if !cli.IsUsageError(err) {
		t.Fatalf("Expected usage error, got: %v", err)
	}

	if auth.User != "user" || auth.Pass != "abc123" {
		t.Errorf("Expected command to retain auth: %+v", auth)
	}
}

func TestLogoutCommandAbort(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	// Fire up test server (should not be called)
	server := testutil.APIServer(t, "GET", "/", "", 200)
	defer server.Close()

	term.SetPromptResponses(map[string]string{
		"Are you sure you want to logout? [y/N]": "ABORT",
	})

	cc := cli.TestContext(term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	err := runCommandNoErr(cc, []string{"logout"})
	if err != nil {
		t.Error(err)
	}

	outStr := string(term.OutBytes())
	if exp := ""; outStr != exp {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	}

	if auth.User != "user" || auth.Pass != "abc123" || auth.Err != nil {
		t.Errorf("Expected command to retain auth: %+v", auth)
	}
}
