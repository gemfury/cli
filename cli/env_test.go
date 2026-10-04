package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"bufio"
	"net/http"
	"strings"
	"testing"
)

// packagesRequest runs "packages" with the given flags, and without a
// terminal to login from, then returns the token and the account that
// its API request was sent with
func packagesRequest(t *testing.T, auth terminal.Auther, flags []string) (token, account string) {
	t.Helper()

	var sent bool
	server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
		h.HandleFunc("/packages", func(w http.ResponseWriter, r *http.Request) {
			token, account = r.Header.Get("Authorization"), r.URL.Query().Get("as")
			w.Write([]byte("[]"))
			sent = true
		})
	})

	term := terminal.NewForTest()
	term.SetInteractive(false)

	cc := testContext(t, term, auth, server)
	if err := runCommandNoErr(cc, append([]string{"packages"}, flags...)); err != nil {
		t.Fatal(err)
	} else if !sent {
		t.Fatal("Expected an API request, got none")
	}

	return token, account
}

// The token is resolved from --api-token, then FURY_TOKEN, then the session
// that "login" has saved
func TestAuthTokenPrecedence(t *testing.T) {
	saved := terminal.TestAuther("user", "saved-token", nil)
	unused := unusedAuther()

	for name, tc := range map[string]struct {
		auth  terminal.Auther
		env   string
		flags []string
		exp   string
	}{
		"flag over env":    {unused, "env-token", []string{"--api-token", "flag-token"}, "flag-token"},
		"env over saved":   {unused, "env-token", nil, "env-token"},
		"env is trimmed":   {unused, "env-token\n", nil, "env-token"},
		"saved, no env":    {saved, "", nil, "saved-token"},
		"saved, blank env": {saved, " \n", nil, "saved-token"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FURY_TOKEN", tc.env)

			if got, _ := packagesRequest(t, tc.auth, tc.flags); got != tc.exp {
				t.Errorf("Expected token %q, got %q", tc.exp, got)
			}
		})
	}
}

// With --api-token -, the token is the first line of stdin, ahead of
// FURY_TOKEN. Without one, nothing is requested, as that is a usage error.
func TestAuthTokenFromStdin(t *testing.T) {
	t.Setenv("FURY_TOKEN", "env-token")

	for name, tc := range map[string]struct {
		stdin string
		exp   string // The token sent, or none for a usage error
	}{
		"piped":      {"stdin-token\n", "stdin-token"},
		"no newline": {"stdin-token", "stdin-token"},
		"padded":     {"  stdin-token \n", "stdin-token"},
		"one line":   {"stdin-token\nsecond-line\n", "stdin-token"},
		"nothing":    {"", ""},
		"blank":      {"  \n", ""},
		"dash":       {"-\n", ""}, // Would be read again
		"too long":   {strings.Repeat("a", bufio.MaxScanTokenSize+1) + "\n", ""},
	} {
		t.Run(name, func(t *testing.T) {
			var sent string
			server := testutil.APIServerCustom(t, func(h *http.ServeMux) {
				h.HandleFunc("/packages", func(w http.ResponseWriter, r *http.Request) {
					sent = r.Header.Get("Authorization")
					w.Write([]byte("[]"))
				})
			})

			term := terminal.NewForTest()
			term.SetInteractive(false)
			term.InWrite([]byte(tc.stdin))

			cc := testContext(t, term, unusedAuther(), server)
			err := runCommand(cc, []string{"packages", "--api-token", "-"})
			if tc.exp == "" {
				if !cli.IsUsageError(err) {
					t.Errorf("Expected a usage error, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if sent != tc.exp {
				t.Errorf("Expected token %q to be sent, got %q", tc.exp, sent)
			}
		})
	}
}

// Backing out of the token prompt interrupts the command, and a blank
// answer is a usage error, as is "-". Nothing is requested either way,
// and the saved session is kept.
func TestAuthTokenNotGiven(t *testing.T) {
	for _, args := range [][]string{{"packages", "--api-token", "-"}, {"login", "--api-token", "-"}} {
		for name, tc := range map[string]struct {
			answer string
			usage  bool // Or else interrupted
		}{
			"interrupted": {"INTERRUPT", false},
			"closed":      {"EOF", false},
			"blank":       {"  ", true},
			"dash":        {"-", true},
		} {
			t.Run(args[0]+" "+name, func(t *testing.T) {
				auth := terminal.TestAuther("old@example.com", "old-token", nil)
				term := terminal.NewForTest()
				term.SetPromptResponses(map[string]string{"API token: ": tc.answer})

				cc := testContext(t, term, auth, offlineServer(t))
				err := runCommand(cc, args)
				if !tc.usage {
					expectUnconfirmed(t, term, err, true)
				} else if !cli.IsUsageError(err) {
					t.Errorf("Expected a usage error, got %v", err)
				}

				expectCredentials(t, auth, "old@example.com", "old-token")
			})
		}
	}
}

// The account is resolved from --account, then FURY_ACCOUNT
func TestAccountPrecedence(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)

	for name, tc := range map[string]struct {
		env   string
		flags []string
		exp   string
	}{
		"flag over env":        {"env-org", []string{"--account", "flag-org"}, "flag-org"},
		"legacy flag over env": {"env-org", []string{"--as", "flag-org"}, "flag-org"},
		"env":                  {"env-org", nil, "env-org"},
		"neither":              {"", nil, ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FURY_ACCOUNT", tc.env)

			if _, got := packagesRequest(t, auth, tc.flags); got != tc.exp {
				t.Errorf("Expected account %q, got %q", tc.exp, got)
			}
		})
	}
}
