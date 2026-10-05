package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// respondInTurn answers each request with the next status, and the last
// one from then on, counting the requests. A failure asks for no wait
// before the request is sent again, by Retry-After: 0, and a 200 has an
// empty listing.
func respondInTurn(requests *atomic.Int32, statuses ...int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := int(requests.Add(1)) - 1
		if status := statuses[min(n, len(statuses)-1)]; status != 200 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(status)
			return
		}
		w.Write([]byte("[]"))
	}
}

// A request that reads is sent again when it is rate limited or the server
// is unavailable, as many times as FURY_RETRIES says, with a notice of each
// wait on stderr unless --quiet. A bug of the server is not retried.
func TestRetries(t *testing.T) {
	const (
		rateLimited = "Rate limited. Retrying in 0s\n"
		unavailable = "Unavailable (HTTP 503). Retrying in 0s\n"
		badGateway  = "Unavailable (HTTP 502). Retrying in 0s\n"
	)

	for name, tc := range map[string]struct {
		env      string // FURY_RETRIES, or the default when empty
		statuses []int
		flags    []string
		requests int32
		exp      int
		stderr   string
	}{
		"rate limited, then served":      {"", []int{429, 200}, nil, 2, cli.ExitOK, rateLimited},
		"unavailable, then served":       {"", []int{503, 503, 200}, nil, 3, cli.ExitOK, unavailable + unavailable},
		"gateway timed out, then served": {"", []int{504, 200}, nil, 2, cli.ExitOK, "Unavailable (HTTP 504). Retrying in 0s\n"},
		"unavailable throughout":         {"", []int{502}, nil, 4, cli.ExitUnavailable, badGateway + badGateway + badGateway + "Error: Something went wrong. Please contact support. (HTTP 502)\n"},
		"a bug is not retried":           {"", []int{500}, nil, 1, cli.ExitUnavailable, "Error: Something went wrong. Please contact support. (HTTP 500)\n"},
		"no retries":                     {"0", []int{429, 200}, nil, 1, cli.ExitUnavailable, "Error: Too many requests. Try again later.\n"},
		"one retry":                      {"1", []int{429, 429, 200}, nil, 2, cli.ExitUnavailable, rateLimited + "Error: Too many requests. Try again later.\n"},
		"quiet":                          {"", []int{429, 200}, []string{"--quiet"}, 2, cli.ExitOK, ""},
		"json":                           {"", []int{429, 200}, []string{"--json"}, 2, cli.ExitOK, ""},
		"not a count":                    {"x", []int{200}, nil, 0, cli.ExitUsage, "Error: FURY_RETRIES must be a count, not \"x\"\n"},
		"not a count, negative":          {"-1", []int{200}, nil, 0, cli.ExitUsage, "Error: FURY_RETRIES must be a count, not \"-1\"\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FURY_RETRIES", tc.env)
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			var requests atomic.Int32
			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("GET /packages", respondInTurn(&requests, tc.statuses...))
			})

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, slices.Concat([]string{"packages"}, tc.flags))
			expectExitStatus(t, term, err, tc.exp)
			expectProblems(t, term, tc.stderr)
			if n := requests.Load(); n != tc.requests {
				t.Errorf("Expected %d requests, got %d", tc.requests, n)
			}
		})
	}
}

// Help is given whatever FURY_RETRIES holds, as it sends no request
func TestHelpIgnoresRetriesEnv(t *testing.T) {
	t.Setenv("FURY_RETRIES", "x")
	if out := helpOutput(t, "help", "packages"); !strings.Contains(out, "fury packages [flags]") {
		t.Errorf("Expected the help of packages, got %q", out)
	}
}

// A request that cannot connect is sent again too, after a second
func TestRetriesConnectionFailed(t *testing.T) {
	t.Setenv("FURY_RETRIES", "1")
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	server := offlineServer(t)
	server.Close() // Nothing listens any more

	cc := testContext(t, term, auth, server)
	expectExitStatus(t, term, runCommand(cc, []string{"packages"}), cli.ExitUnavailable)
	expectProblems(t, term, "Connection failed. Retrying in 1s\nError: ")
}

// A request that writes is never sent again, as its body cannot be replayed
func TestRetriesNotOnWrite(t *testing.T) {
	t.Setenv("FURY_RETRIES", "")
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	var requests atomic.Int32
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /uploads", respondInTurn(&requests, 503))
	})

	cc := testContext(t, term, auth, server)
	expectExitStatus(t, term, runCommand(cc, []string{"push", samplePackagePath()}), cli.ExitUnavailable)
	expectErrOutput(t, term, "Error: Something went wrong. Please contact support. (HTTP 503)\n")
	if n := requests.Load(); n != 1 {
		t.Errorf("Expected 1 request, got %d", n)
	}
}

// A Retry-After header is waited for, in seconds or until a date, up to a
// minute; without one, the wait is a second, then double that. A wait of
// a minute is not waited out: the command is interrupted once the wait is
// announced.
func TestRetryAfter(t *testing.T) {
	for name, tc := range map[string]struct {
		after string        // The header, or none when empty
		wait  time.Duration // As announced
	}{
		"seconds":     {"1", time.Second},
		"negative":    {"-5", 0},
		"a past date": {time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), 0},
		"no header":   {"", time.Second},
		"capped":      {"3600", time.Minute},
		"a far date":  {time.Now().Add(time.Hour).UTC().Format(http.TimeFormat), time.Minute},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FURY_RETRIES", "")
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			var requests atomic.Int32
			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("GET /packages", func(w http.ResponseWriter, r *http.Request) {
					if requests.Add(1) > 1 {
						w.Write([]byte("[]"))
						return
					}
					if tc.after != "" {
						w.Header().Set("Retry-After", tc.after)
					}
					w.WriteHeader(429)
				})
			})

			cc, cancel := context.WithCancel(testContext(t, term, auth, server))
			defer cancel()
			interrupt := tc.wait == time.Minute
			if interrupt {
				term.OnErrOutput(cancel) // Once the wait is announced
			}

			start := time.Now()
			err := runCommand(cc, []string{"packages"})
			notice := fmt.Sprintf("Rate limited. Retrying in %s\n", tc.wait)
			if interrupt {
				expectInterrupted(t, err)
				expectProblems(t, term, notice, "Cancelled\n")
				if n := requests.Load(); n != 1 {
					t.Errorf("Expected 1 request, got %d", n)
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}
			expectErrOutput(t, term, notice)
			if waited := time.Since(start) >= time.Second; waited != (tc.wait > 0) {
				t.Errorf("Expected to wait %s, took %s", tc.wait, time.Since(start))
			}
		})
	}
}

// A request that takes longer than --timeout fails, as unavailable, and
// is not sent again, even with retries on
func TestTimeout(t *testing.T) {
	for name, args := range map[string][]string{
		"request":  {"versions", "foo"},
		"transfer": {"push", samplePackagePath()},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FURY_RETRIES", "3")
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			var requests atomic.Int32
			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/", stall(func() { requests.Add(1) }))
			})

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, slices.Concat(args, []string{"--timeout", "50ms"}))
			expectExitStatus(t, term, err, cli.ExitUnavailable)
			expectErrOutput(t, term, "Error: Operation timed out. Try again later.\n")
			if n := requests.Load(); n != 1 {
				t.Errorf("Expected 1 request, got %d", n)
			}
		})
	}
}

// A negative --timeout is a usage error, before anything is requested
func TestTimeoutNegative(t *testing.T) {
	expectUsageError(t, []string{"versions", "foo", "--timeout", "-1s"}, "--timeout cannot be negative: -1s")
}

// A request has 30s by default, and a transfer 10m
func TestTimeoutDefaults(t *testing.T) {
	cc := testContext(t, terminal.NewForTest(), terminal.TestAuther("user", "abc123", nil), offlineServer(t))
	if request, long := cli.ClientTimeouts(t, cc); request != 30*time.Second || long != 10*time.Minute {
		t.Errorf("Expected 30s and 10m, got %s and %s", request, long)
	}
}
