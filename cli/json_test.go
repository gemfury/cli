package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// jsonCase is a command that takes --json: the arguments after its name,
// and a server with the fixtures of its own test file
type jsonCase struct {
	args   []string
	server func(*testing.T) *httptest.Server
}

// jsonCases has each such command by its path after "fury"
var jsonCases = map[string]jsonCase{
	"packages":       {nil, serving("GET", "/packages", 200, packagesResponses...)},
	"versions":       {[]string{"foo"}, serving("GET", "/packages/foo/versions", 200, versionsResponses...)},
	"sharing":        {nil, serving("GET", "/members", 200, sharingResponses...)},
	"accounts":       {nil, serving("GET", "/collaborations", 200, accountsResponses...)},
	"git list":       {nil, serving("GET", "/git/repos/me", 200, gitReposResponses...)},
	"git config":     {[]string{"repo-name"}, serving("GET", gitConfigPath, 200, gitConfigResponse)},
	"git config get": {[]string{"repo-name", "KEY2"}, serving("GET", gitConfigPath, 200, gitConfigResponse)},
	"git stack":      {[]string{"repo-name"}, testGitStackServer},
	"whoami":         {nil, serving("GET", "/users/me", 200, whoamiResponse)},
}

// jsonOutputs has what each command of jsonCases prints with --json
var jsonOutputs = map[string]string{
	"packages": `[
  {
    "id": "pkg_a1b2c3",
    "name": "pkg-ruby",
    "kind_key": "ruby",
    "private": false,
    "version_count": 3,
    "latest_version": {
      "id": "ver_r1r2r3",
      "version": "1.2.0.pre",
      "prerelease": true
    },
    "release_version": {
      "id": "ver_q1q2q3",
      "version": "1.1.1",
      "prerelease": false
    }
  },
  {
    "id": "pkg_z1y2x3",
    "name": "pkg-js",
    "kind_key": "js",
    "private": true,
    "version_count": 1,
    "latest_version": {
      "id": "ver_j1j2j3",
      "version": "0.1.0-beta.1",
      "prerelease": true
    },
    "release_version": null
  }
]
`,
	"versions": `[
  {
    "id": "ver_a1b2c3",
    "version": "1.2.3",
    "prerelease": false,
    "kind_key": "js",
    "digests": {
      "md5": "0123abcd",
      "sha1": "4567ef01",
      "sha256": "89ab2345",
      "sha512": "cdef6789"
    },
    "filename": "foo-1.2.3.tgz",
    "created_at": "2011-05-27T00:39:07Z",
    "created_by": {
      "id": "acct_d4e5f6",
      "name": "user1"
    },
    "package": {
      "id": "pkg_x9y8z7",
      "name": "foo",
      "kind_key": "js",
      "private": true,
      "version_count": 2
    }
  },
  {
    "id": "ver_z1y2x3",
    "version": "3.2.1",
    "prerelease": false,
    "kind_key": "js",
    "digests": {
      "md5": "fedc3210",
      "sha1": "ba987654",
      "sha256": "76543210",
      "sha512": "3210fedc"
    },
    "filename": "foo-3.2.1.tgz",
    "created_at": "2011-01-27T00:44:00Z",
    "created_by": {
      "id": "acct_g7h8i9",
      "name": "user2"
    },
    "package": {
      "id": "pkg_x9y8z7",
      "name": "foo",
      "kind_key": "js",
      "private": true,
      "version_count": 2
    }
  }
]
`,
	"sharing": `[
  {
    "id": "acct_a1b2c3",
    "name": "test-name",
    "type": "",
    "username": "test-user",
    "role": "owner"
  },
  {
    "id": "acct_z1y2x3",
    "name": "collaborator",
    "type": "",
    "username": "test-collab",
    "role": "push"
  }
]
`,
	"accounts": `[
  {
    "id": "acct_a1b2c3",
    "name": "my-name",
    "type": "",
    "username": "test-self",
    "role": "owner"
  },
  {
    "id": "acct_z1y2x3",
    "name": "org-name",
    "type": "",
    "username": "test-org",
    "role": "push"
  }
]
`,
	"git list": `[
  {
    "id": "repo_a1b2c3",
    "name": "repoA",
    "build_stack": {
      "name": "fury-14"
    }
  },
  {
    "id": "repo_z1y2x3",
    "name": "repoZ",
    "build_stack": {
      "name": "fury-22"
    }
  }
]
`,
	"git config": `{
  "KEY1": "VALUE1",
  "KEY2": "VALUE2"
}
`,
	"git config get": `{
  "KEY2": "VALUE2"
}
`,
	"git stack": `[
  {
    "name": "fury-14",
    "current": true
  },
  {
    "name": "fury-22",
    "current": false
  }
]
`,
	"whoami": `{
  "id": "acct_j0e1t2",
  "name": "joetest",
  "type": "user",
  "email": "joe@example.com",
  "username": "joetest"
}
`,
}

// expectJSON runs the command of name with its arguments, flags, and
// --json, as a user who is logged in, and asserts that stdout has
// exactly stdout, and stderr nothing
func expectJSON(t *testing.T, name string, tc jsonCase, stdout string, flags ...string) {
	t.Helper()
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	cc := testContext(t, term, auth, tc.server(t))
	args := slices.Concat(strings.Fields(name), tc.args, flags, []string{"--json"})
	if err := runCommand(cc, args); err != nil {
		t.Fatal(err)
	}

	expectOutput(t, term, stdout, "")
	if bars, spinners := term.ProgressShown(); bars != 0 || spinners != 0 {
		t.Errorf("JSON showed progress: %d bars, %d spinners", bars, spinners)
	}
}

// With --json, a result is one JSON document on stdout and nothing else,
// and with --quiet the output is the same
func TestJSONOutput(t *testing.T) {
	for name, tc := range jsonCases {
		t.Run(name, func(t *testing.T) { expectJSON(t, name, tc, jsonOutputs[name]) })
	}

	t.Run("with --quiet", func(t *testing.T) {
		expectJSON(t, "sharing", jsonCases["sharing"], jsonOutputs["sharing"], "--quiet")
	})
}

// With --json, a listing of nothing is an empty array, or an empty
// object for the keys of "git config", and not a message
func TestJSONEmpty(t *testing.T) {
	for name, tc := range map[string]struct {
		jsonCase
		stdout string
	}{
		"packages":       {jsonCase{nil, serving("GET", "/packages", 200, "[]")}, "[]\n"},
		"versions":       {jsonCase{[]string{"foo"}, serving("GET", "/packages/foo/versions", 200, "[]")}, "[]\n"},
		"sharing":        {jsonCase{nil, serving("GET", "/members", 200, "[]")}, "[]\n"},
		"accounts":       {jsonCase{nil, serving("GET", "/collaborations", 200, "[]")}, "[]\n"},
		"git list":       {jsonCase{nil, serving("GET", "/git/repos/me", 200, `{"repos": []}`)}, "[]\n"},
		"git config":     {jsonCase{[]string{"repo-name"}, serving("GET", gitConfigPath, 200, `{"config_vars": {}}`)}, "{}\n"},
		"git config get": {jsonCase{[]string{"repo-name", "NOSUCH"}, serving("GET", gitConfigPath, 200, gitConfigResponse)}, "{}\n"},
	} {
		t.Run(name, func(t *testing.T) { expectJSON(t, name, tc.jsonCase, tc.stdout) })
	}
}

// failsAfterFirstPage serves one page of collaborators, and refuses the next
func failsAfterFirstPage(t *testing.T) *httptest.Server {
	t.Helper()
	var requests atomic.Int32
	return testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/members", func(w http.ResponseWriter, r *http.Request) {
			if requests.Add(1) > 1 {
				respondWith(403, "")(w, r)
				return
			}
			testutil.APIPaginatedResponse(t, w, r, sharingResponses, 200)
		})
	})
}

// With --json, a failure is reported as without it, on stderr, and stdout
// has nothing: not the pages that were fetched before it, and not the
// questions of a login, which is not started.
func TestJSONFailure(t *testing.T) {
	const forbidden = "You're not allowed to do this"

	for name, tc := range map[string]struct {
		command string
		token   string
		server  func(*testing.T) *httptest.Server
		msg     string
		exp     int
	}{
		"forbidden":   {"sharing", "abc123", serving("GET", "/members", 403, "[]"), forbidden, cli.ExitError},
		"not found":   {"git config repo-name", "abc123", serving("GET", gitConfigPath, 404, ""), `Repository "repo-name": Doesn't look like this exists`, cli.ExitNotFound},
		"second page": {"sharing", "abc123", failsAfterFirstPage, forbidden, cli.ExitError},
		"logged out":  {"whoami", "", offlineServer, cli.ErrNotLoggedIn.Error(), cli.ExitAuth},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("", tc.token, nil)
			term := terminal.NewForTest()

			cc := testContext(t, term, auth, tc.server(t))
			err := runCommand(cc, append(strings.Fields(tc.command), "--json"))
			expectExitStatus(t, term, err, tc.exp)
			expectOutput(t, term, "", "Error: "+tc.msg+"\n")
		})
	}
}

// A command takes --json if and only if jsonCases has it, with an output
// in jsonOutputs. Any other command, the root included, refuses the flag
// as unknown: a usage error before its arguments are checked, and before
// it asks or requests anything.
func TestJSONCommands(t *testing.T) {
	for _, cmd := range allCommands(rootForTest(t)) {
		path := cmd.CommandPath()
		t.Run(path, func(t *testing.T) {
			_, name, _ := strings.Cut(path, " ") // Past "fury", as in jsonCases
			takesJSON := cmd.Flags().Lookup("json") != nil
			_, hasCase := jsonCases[name]
			_, hasOutput := jsonOutputs[name]
			if takesJSON != hasCase || hasCase != hasOutput {
				t.Errorf("Takes --json: %v, in jsonCases: %v, in jsonOutputs: %v", takesJSON, hasCase, hasOutput)
			}

			if !takesJSON {
				expectUsageError(t, append(strings.Fields(name), "--json"), "unknown flag: --json")
			}
		})
	}
}

// Only the documented fields are printed, so that neither secrets nor
// fields that the API adds later leak into --json. Values are not HTML
// escaped, and a terminal escape is encoded. The output is compared
// compacted, as one document with the newline that shell consumers expect.
func TestJSONApprovedFields(t *testing.T) {
	for _, tc := range []struct {
		name, command, path, body, want string
	}{
		{"package", "packages", "/packages", `[{"name":"foo","token":"secret","future":"hidden","latest_version":{"version":"1","download_url":"secret"}}]`, `[{"id":"","name":"foo","kind_key":"","private":false,"version_count":0,"latest_version":{"id":"","version":"1","prerelease":false},"release_version":null}]`},
		// An older API gives the kind on the package alone: it is printed on the version too
		{"version", "versions foo", "/packages/foo/versions", `[{"version":"1","download_url":"secret","token":"secret","future":"hidden","created_by":{"name":"user","email":"secret"},"package":{"name":"foo","kind_key":"js","future":"hidden"},"digests":{"sha256":"abc","future":"hidden"}}]`, `[{"id":"","version":"1","prerelease":false,"kind_key":"js","digests":{"md5":"","sha1":"","sha256":"abc","sha512":""},"filename":"","created_at":"0001-01-01T00:00:00Z","created_by":{"id":"","name":"user"},"package":{"id":"","name":"foo","kind_key":"js","private":false,"version_count":0}}]`},
		{"account", "whoami", "/users/me", `{"name":"雪<&>\n\u001b","token":"secret","future":"hidden"}`, `{"id":"","name":"雪<&>\n\u001b","type":"","email":"","username":""}`},
		{"member", "sharing", "/members", `[{"role":"owner","email":"secret","token":"secret","future":"hidden"}]`, `[{"id":"","name":"","type":"","username":"","role":"owner"}]`},
		{"repository", "git list", "/git/repos/me", `{"repos":[{"name":"repo","token":"secret","build_stack":{"name":"stack","future":"hidden"}}]}`, `[{"id":"","name":"repo","build_stack":{"name":"stack"}}]`},
		{"configuration", "git config get repo-name EMPTY TOKEN MISSING", gitConfigPath, `{"config_vars":{"EMPTY":"","TOKEN":"secret<&>\n雪","IGNORED":"value","NULL":null}}`, `{"EMPTY":"","TOKEN":"secret<&>\n雪"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			term := terminal.NewForTest()
			cc := testContext(t, term, terminal.TestAuther("user", "abc123", nil), serving("GET", tc.path, 200, tc.body)(t))
			if err := runCommand(cc, append(strings.Fields(tc.command), "--json")); err != nil {
				t.Fatal(err)
			}

			output := term.OutBytes()
			var compact bytes.Buffer
			if err := json.Compact(&compact, output); err != nil {
				t.Fatalf("Invalid JSON %q: %v", output, err)
			}
			if compact.String() != tc.want {
				t.Errorf("JSON = %s, want %s", compact.Bytes(), tc.want)
			}
			if !bytes.HasSuffix(output, []byte("\n")) {
				t.Error("JSON has no trailing newline")
			}
		})
	}
}
