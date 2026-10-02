package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/pkg/terminal"

	"errors"
	"strings"
	"testing"
)

// What Git asks its credential helper for a push to Gemfury
const gitCredentialRequest = "protocol=https\nhost=git.fury.io\n\n"

// gitCredentials runs the credential helper as Git does,
// with what it asks on stdin, and without the API to ask
func gitCredentials(t *testing.T, auth terminal.Auther, operation, request string) (terminal.TestTerm, error) {
	t.Helper()
	term := terminal.NewForTest()
	term.InWrite([]byte(request))

	cc := testContext(t, term, auth, offlineServer(t))
	return term, runCommand(cc, []string{"git", "credentials", operation})
}

// Git is given the saved session for git.fury.io,
// and nothing for any other host
func TestGitCredentialsGet(t *testing.T) {
	answer := "protocol=https\nhost=git.fury.io\nusername=u@example.com\npassword=abc123\n"

	for name, tc := range map[string]struct {
		request, stdout string
	}{
		"our host":         {gitCredentialRequest, answer},
		"without the end":  {"protocol=https\nhost=git.fury.io", answer},
		"with a path":      {"protocol=https\nhost=git.fury.io\npath=me/repo.git\n\n", answer},
		"for our user":     {"protocol=https\nhost=git.fury.io\nusername=u@example.com\n\n", answer},
		"for another user": {"protocol=https\nhost=git.fury.io\nusername=o@example.com\n\n", ""},
		"another host":     {"protocol=https\nhost=github.com\n\n", ""},
		"a host like ours": {"protocol=https\nhost=git.fury.io.example.com\n\n", ""},
		"not by HTTPS":     {"protocol=http\nhost=git.fury.io\n\n", ""},
		"nothing asked":    {"", ""},

		// A carriage return is part of its value, and never ends a line
		"carriage returns": {"protocol=https\r\nhost=git.fury.io\r\n\r\n", ""},
		"smuggled host":    {"protocol=https\nhost=github.com\rhost=git.fury.io\n\n", ""},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("u@example.com", "abc123", nil)
			term, err := gitCredentials(t, auth, "get", tc.request)
			if err != nil {
				t.Errorf("Command error: %s", err)
			}

			expectOutput(t, term, tc.stdout, "")
		})
	}
}

// Without a session, Git is left to ask for the credentials
func TestGitCredentialsLoggedOut(t *testing.T) {
	term, err := gitCredentials(t, terminal.TestAuther("", "", nil), "get", gitCredentialRequest)
	if err != nil {
		t.Errorf("Command error: %s", err)
	}

	expectOutput(t, term, "", "")
}

// A session that cannot be read is an error, for Git to show
func TestGitCredentialsUnreadable(t *testing.T) {
	cause := errors.New("unreadable")
	term, err := gitCredentials(t, terminal.TestAuther("", "", cause), "get", gitCredentialRequest)
	if !errors.Is(err, cause) {
		t.Errorf("Expected the failure to read, got: %v", err)
	}

	expectOutput(t, term, "", "Error: unreadable\n")
}

// Git neither saves the session nor removes it: only login and logout do.
// The saved session is not even read.
func TestGitCredentialsIgnored(t *testing.T) {
	for _, operation := range []string{"store", "erase", "capability"} {
		t.Run(operation, func(t *testing.T) {
			request := "protocol=https\nhost=git.fury.io\nusername=u@example.com\npassword=abc123\n\n"
			term, err := gitCredentials(t, unusedAuther(), operation, request)
			if err != nil {
				t.Errorf("Command error: %s", err)
			}

			expectOutput(t, term, "", "")
		})
	}
}

// The helper that login sets for Git is a command of this CLI
func TestGitCredentialHelperCommand(t *testing.T) {
	cc := cli.TestContext(t.Context(), terminal.NewForTest(), terminal.TestAuther("", "", nil))
	args := append(strings.Fields(terminal.GitCredentialHelper), "get")

	found, err := exampleCommand(cc, args)
	if err != nil {
		t.Fatal(err)
	} else if exp := "fury git credentials"; found.CommandPath() != exp {
		t.Errorf("Expected the helper to be %q, got %q", exp, found.CommandPath())
	} else if !found.Hidden {
		t.Error("Expected the helper to be hidden from help")
	}
}
