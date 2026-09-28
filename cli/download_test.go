package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const downloadContent = "package-bytes-0123456789"

// Of a file that is there already, and fails its checksum
const staleContent = "stale-bytes"

// Version JSON whose download_url points back at the test server.
// The host is only known per-request, so it is rendered by the handler.
func downloadVersionJSON(r *http.Request, id, pkg, ver, filename string) string {
	sum := sha512.Sum512([]byte(downloadContent))
	return fmt.Sprintf(`{
		"id": %q, "version": %q, "filename": %q,
		"created_at": "2011-05-27T00:39:00Z",
		"download_url": "http://%s/downloads/%s",
		"digests": { "sha512": %q },
		"package": { "id": "pkg_1", "name": %q, "kind_key": "js" }
	}`, id, ver, filename, r.Host, filename, hex.EncodeToString(sum[:]), pkg)
}

// downloadVersionHandler responds with version 1.2.3 of the package asked for
func downloadVersionHandler(w http.ResponseWriter, r *http.Request) {
	pkg := r.PathValue("pkg")
	w.Write([]byte(downloadVersionJSON(r, "ver_"+pkg, pkg, "1.2.3", pkg+"-1.2.3.tgz")))
}

func downloadHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if a := r.Header.Get("Authorization"); a != "abc123" {
			t.Errorf("Download should be authenticated, got %q", a)
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(downloadContent)))
		w.Write([]byte(downloadContent))
	}
}

// ==== beta download ====

func TestDownloadCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/packages/{pkg}/versions/1.2.3", downloadVersionHandler)
		mux.HandleFunc("/downloads/", downloadHandler(t))
	})

	cc := testContext(t, term, auth, server)

	// A trailing slash on a configured endpoint must be tolerated
	ctx.GlobalFlags(cc).Endpoint = server.URL + "/"

	// Download writes to the current directory
	t.Chdir(t.TempDir())

	if err := runCommandNoErr(cc, []string{"beta", "download", "foo@1.2.3"}); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile("foo-1.2.3.tgz")
	if err != nil {
		t.Fatalf("Downloaded file: %s", err)
	} else if string(body) != downloadContent {
		t.Errorf("Downloaded content %q, expected %q", body, downloadContent)
	}

	if outStr := string(term.OutBytes()); !strings.Contains(outStr, "💾 foo-1.2.3.tgz") {
		t.Errorf("Expected saved status in output, got %q", outStr)
	}
}

// A download_url on a foreign host is refused rather than sent our token
func TestDownloadCommandForeignURL(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/packages/foo/versions/1.2.3", func(w http.ResponseWriter, r *http.Request) {
			v := downloadVersionJSON(r, "ver_1", "foo", "1.2.3", "foo-1.2.3.tgz")
			v = strings.Replace(v, "http://"+r.Host, "https://evil.example.com", 1)
			w.Write([]byte(v))
		})
	})

	cc := testContext(t, term, auth, server)
	t.Chdir(t.TempDir())

	err := runCommand(cc, []string{"beta", "download", "foo@1.2.3"})
	if err == nil || !strings.Contains(err.Error(), "not served by") {
		t.Fatalf("Expected foreign URL error, got: %v", err)
	}

	if _, err := os.Stat("foo-1.2.3.tgz"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("No file should be written for a refused download")
	}
}

// An interrupted download leaves no partial file to fail a later checksum
func TestDownloadCommandInterrupted(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	cc, cancel := context.WithCancel(cli.TestContext(t.Context(), term, auth))
	defer cancel()

	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/packages/{pkg}/versions/1.2.3", downloadVersionHandler)
		mux.HandleFunc("/downloads/", func(w http.ResponseWriter, r *http.Request) {
			// Interrupted after the first half of the file
			w.Header().Set("Content-Length", fmt.Sprint(len(downloadContent)))
			w.Write([]byte(downloadContent[:len(downloadContent)/2]))
			w.(http.Flusher).Flush()
			cancel()
			<-r.Context().Done()
		})
	})

	flags := ctx.GlobalFlags(cc)
	flags.Endpoint = server.URL

	t.Chdir(t.TempDir())

	err := runCommand(cc, []string{"beta", "download", "foo@1.2.3"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}

	if _, err := os.Stat("foo-1.2.3.tgz"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("No file should be left by an interrupted download")
	}
}

func TestDownloadCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/packages/foo/versions/1.2.3", "", 403)
	testCommandForbiddenResponse(t, []string{"beta", "download", "foo@1.2.3"}, server, `Version "foo@1.2.3"`)
}

// ==== beta backup ====

func TestBackupCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	var downloads int
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/versions/$dump", func(w http.ResponseWriter, r *http.Request) {
			if k := r.URL.Query().Get("kind"); k != "js" {
				t.Errorf("Expected kind filter 'js', got %q", k)
			}
			w.Write([]byte("[" +
				downloadVersionJSON(r, "ver_1", "foo", "1.2.3", "foo-1.2.3.tgz") + "," +
				downloadVersionJSON(r, "ver_2", "@scope/bar", "0.1.0", "bar-0.1.0.tgz") +
				"]"))
		})
		mux.HandleFunc("/downloads/", func(w http.ResponseWriter, r *http.Request) {
			downloads++
			downloadHandler(t)(w, r)
		})
	})

	cc := testContext(t, term, auth, server)

	dest := t.TempDir()
	if err := runCommandNoErr(cc, []string{"beta", "backup", "--kind", "js", dest}); err != nil {
		t.Fatal(err)
	}

	// Files land in KIND/PACKAGE/ID_FILENAME, with path separators neutralized
	expected := []string{
		filepath.Join("js", "foo", "ver_1_foo-1.2.3.tgz"),
		filepath.Join("js", "@scope_bar", "ver_2_bar-0.1.0.tgz"),
	}
	for _, rel := range expected {
		body, err := os.ReadFile(filepath.Join(dest, rel))
		if err != nil {
			t.Errorf("Backup file %s: %s", rel, err)
		} else if string(body) != downloadContent {
			t.Errorf("Backup file %s has content %q", rel, body)
		}
	}

	if downloads != 2 {
		t.Errorf("Expected 2 downloads, got %d", downloads)
	}

	// A second run verifies checksums and downloads nothing
	if err := runCommandNoErr(cc, []string{"beta", "backup", "--kind", "js", dest}); err != nil {
		t.Fatal(err)
	}

	if downloads != 2 {
		t.Errorf("Expected no new downloads on second run, got %d total", downloads)
	}

	if outStr := string(term.OutBytes()); strings.Count(outStr, "✅") != 2 {
		t.Errorf("Expected 2 checksum-verified lines, got %q", outStr)
	}
}

// A failed download stops the backup, and is about its version
func TestBackupCommandForbidden(t *testing.T) {
	var downloads int
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/versions/$dump", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("[" +
				downloadVersionJSON(r, "ver_1", "foo", "1.2.3", "foo-1.2.3.tgz") + "," +
				downloadVersionJSON(r, "ver_2", "bar", "0.1.0", "bar-0.1.0.tgz") +
				"]"))
		})
		mux.HandleFunc("/downloads/", func(w http.ResponseWriter, r *http.Request) {
			downloads++
			respondWith(403, "")(w, r)
		})
	})

	args := []string{"beta", "backup", t.TempDir()}
	testCommandForbiddenResponse(t, args, server, `Version "foo@1.2.3"`)

	if downloads != 1 {
		t.Errorf("Expected 1 download, got %d", downloads)
	}
}

// A file that is there already, but fails its checksum, is downloaded again
// only when confirmed. Declining fails that item alone, whereas leaving the
// question unanswered interrupts the command, the next item unattempted.
// With --yes, the question is not asked.
func TestDownloadCommandChecksumMismatch(t *testing.T) {
	const declined = "Problem downloading \"foo@1.2.3\": Checksum failed\nError: 1 of 2 downloads failed\n"

	for answer, tc := range map[string]struct {
		foo, bar string // Content of each file afterwards
		stderr   string
	}{
		"Y":         {downloadContent, downloadContent, ""},
		"N":         {staleContent, downloadContent, declined},
		"ABORT":     {staleContent, downloadContent, declined},
		"INTERRUPT": {staleContent, "", "Cancelled\n"},
		"EOF":       {staleContent, "", "Cancelled\n"},
		"--yes":     {downloadContent, downloadContent, ""},
	} {
		t.Run(answer, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/packages/{pkg}/versions/1.2.3", downloadVersionHandler)
				mux.HandleFunc("/downloads/", downloadHandler(t))
			})

			// The answer is given by the flag, or else by the user
			args := []string{"beta", "download", "foo@1.2.3", "bar@1.2.3"}
			if strings.HasPrefix(answer, "--") {
				args = append(args, answer)
			} else {
				term.SetPromptResponses(map[string]string{
					"Do you want to delete and redownload? [y/N]": answer,
				})
			}

			t.Chdir(t.TempDir())
			if err := os.WriteFile("foo-1.2.3.tgz", []byte(staleContent), 0600); err != nil {
				t.Fatal(err)
			}

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, args)
			if (err != nil) != (tc.stderr != "") {
				t.Errorf("Unexpected command error: %v", err)
			}

			expectErrOutput(t, term, tc.stderr)

			for name, exp := range map[string]string{"foo-1.2.3.tgz": tc.foo, "bar-1.2.3.tgz": tc.bar} {
				if body, _ := os.ReadFile(name); string(body) != exp {
					t.Errorf("Expected %q in %s, got %q", exp, name, body)
				}
			}
		})
	}
}

// Without a terminal, a file that fails its checksum is a usage error for
// lack of --yes, and so is the exit status when every item fails that way
func TestDownloadCommandChecksumMismatchUnattended(t *testing.T) {
	for name, tc := range map[string]struct {
		bar string // Version of the second item
		exp int
	}{
		"each unconfirmed": {"1.2.3", cli.ExitUsage},
		"one not found":    {"9.9.9", cli.ExitError},
	} {
		t.Run(name, func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()
			term.SetInteractive(false)

			server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/packages/{pkg}/versions/1.2.3", downloadVersionHandler)
				mux.HandleFunc("/packages/{pkg}/versions/9.9.9", respondWith(404, ""))
			})

			t.Chdir(t.TempDir())
			for _, name := range []string{"foo-1.2.3.tgz", "bar-1.2.3.tgz"} {
				if err := os.WriteFile(name, []byte(staleContent), 0600); err != nil {
					t.Fatal(err)
				}
			}

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, []string{"beta", "download", "foo@1.2.3", "bar@" + tc.bar})
			expectExitStatus(t, term, err, tc.exp)
		})
	}
}

// Versions that are missing: each is reported, in order, then summarized
func TestDownloadCommandFailures(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	server := testutil.APIServer(t, "GET", "/packages/{pkg}/versions/1.2.3", "", 404)

	cc := testContext(t, term, auth, server)
	err := runCommand(cc, []string{"beta", "download", "foo@1.2.3", "bar@1.2.3"})
	expectSummaryError(t, err, api.ErrNotFound, "2 of 2 downloads failed")
	expectProblems(t, term,
		"Problem downloading \"foo@1.2.3\": Doesn't look like this exists\n",
		"Problem downloading \"bar@1.2.3\": Doesn't look like this exists\n",
	)
}
