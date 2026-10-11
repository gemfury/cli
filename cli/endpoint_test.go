package cli_test

import (
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// FURY_API_URL is where the API requests go, below its path if it has one.
// A URL is used as given, trimmed; a host alone is https. HOST stands for
// the host and port of the server, which answers over TLS where https is
// given or assumed.
func TestEndpointFromEnv(t *testing.T) {
	for name, tc := range map[string]struct {
		value string // Of FURY_API_URL
		tls   bool
		path  string // Of the request that arrives
	}{
		"http URL":       {"http://HOST", false, "/packages"},
		"URL slash":      {"http://HOST/", false, "/packages"},
		"URL with path":  {"http://HOST/api/", false, "/api/packages"},
		"URL trimmed":    {" http://HOST\n", false, "/packages"},
		"scheme upcased": {"HTTP://HOST", false, "/packages"},
		"https URL":      {"https://HOST", true, "/packages"},
		"host and port":  {"HOST", true, "/packages"},
		"host slash":     {"HOST/", true, "/packages"},
		"host with path": {"HOST/api", true, "/api/packages"},
		"host trimmed":   {" HOST\n", true, "/packages"},
	} {
		t.Run(name, func(t *testing.T) {
			var host, req string
			handle := func(mux *http.ServeMux) {
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					host, req = r.Host, r.Method+" "+r.URL.Path
					w.Write([]byte("[]"))
				})
			}

			var server *httptest.Server
			if tc.tls {
				server = testutil.APIServerCustomTLS(t, handle)
				trustServer(t, server)
			} else {
				server = testutil.APIServerCustom(t, handle)
			}

			serverHost := server.Listener.Addr().String()
			cc := envContext(t, map[string]string{"FURY_API_URL": strings.ReplaceAll(tc.value, "HOST", serverHost)})
			if err := runCommandNoErr(cc, []string{"packages"}); err != nil {
				t.Fatal(err)
			}

			if exp := "GET " + tc.path; host != serverHost || req != exp {
				t.Errorf("Expected %q at %s, got %q at %s", exp, serverHost, req, host)
			}
		})
	}
}

// FURY_PUSH_URL is where uploads go, while the API has its own endpoint
func TestPushEndpointFromEnv(t *testing.T) {
	server := testutil.APIServer(t, "POST", "/uploads", pushResponse, 200)
	cc := envContext(t, map[string]string{"FURY_PUSH_URL": server.URL, "FURY_API_URL": offlineServer(t).URL})
	if err := runCommandNoErr(cc, []string{"push", samplePackagePath()}); err != nil {
		t.Fatal(err)
	}
}

// The endpoint fields, as a host or a test sets them, take precedence
// over the environment, as a flag does over its variable
func TestEndpointFieldOverEnv(t *testing.T) {
	t.Setenv("FURY_API_URL", offlineServer(t).URL)

	server := testutil.APIServer(t, "GET", "/packages", "[]", 200)
	cc := testContext(t, terminal.NewForTest(), terminal.TestAuther("user", "abc123", nil), server)
	if err := runCommandNoErr(cc, []string{"packages"}); err != nil {
		t.Fatal(err)
	}
}

// A value that is not a URL or a host is a usage error naming the
// variable, before any request
func TestEndpointInvalid(t *testing.T) {
	for env, values := range map[string][]string{
		"FURY_API_URL": {
			"://bad", "ftp://example.com", "https://", "https:///path", "a b",
			"https://example.com?x=1", "https://example.com?", "https://example.com#top",
		},
		"FURY_PUSH_URL": {"://bad"},
	} {
		for _, value := range values {
			t.Run(env+" "+value, func(t *testing.T) {
				cc := envContext(t, map[string]string{env: value})
				msg := fmt.Sprintf("%s must be a URL or host, not %q", env, value)
				expectUsageErrorIn(t, cc, []string{"packages"}, msg)
			})
		}
	}
}
