package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const downloadContent = "package-bytes-0123456789"

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

func downloadHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
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
		mux.HandleFunc("/packages/foo/versions/1.2.3", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(downloadVersionJSON(r, "ver_1", "foo", "1.2.3", "foo-1.2.3.tgz")))
		})
		mux.HandleFunc("/downloads/", downloadHandler(t))
	})
	defer server.Close()

	cc := testContext(term, auth, server)

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
	defer server.Close()

	cc := testContext(term, auth, server)
	t.Chdir(t.TempDir())

	err := runCommand(cc, []string{"beta", "download", "foo@1.2.3"})
	if err == nil || !strings.Contains(err.Error(), "not served by") {
		t.Fatalf("Expected foreign URL error, got: %v", err)
	}

	if _, err := os.Stat("foo-1.2.3.tgz"); !os.IsNotExist(err) {
		t.Errorf("No file should be written for a refused download")
	}
}

func TestDownloadCommandForbidden(t *testing.T) {
	server := testutil.APIServer(t, "GET", "/packages/foo/versions/1.2.3", "", 403)
	defer server.Close()
	testCommandForbiddenResponse(t, []string{"beta", "download", "foo@1.2.3"}, server)
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
	defer server.Close()

	cc := testContext(term, auth, server)

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

// A malformed argument and a missing version: both are reported, in order
func TestDownloadCommandFailures(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	server := testutil.APIServer(t, "GET", "/packages/foo/versions/1.2.3", "", 404)
	defer server.Close()

	cc := testContext(term, auth, server)
	err := runCommand(cc, []string{"beta", "download", "no-version", "foo@1.2.3"})
	expectSummaryError(t, err, api.ErrNotFound, "2 of 2 downloads failed")
	expectProblems(t, term,
		"Problem downloading \"no-version\": Argument format: PACKAGE@VERSION\n",
		"Problem downloading \"foo@1.2.3\": Doesn't look like this exists\n",
	)
}
