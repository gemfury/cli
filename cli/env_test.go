package cli_test

import (
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"net/http"
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

// The token is resolved from --api-token, then FURY_TOKEN, then .netrc
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
