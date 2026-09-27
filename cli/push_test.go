package cli_test

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/internal/testutil"
	"github.com/gemfury/cli/pkg/terminal"

	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
)

const pushResponse = `{}`

// ==== push ====

func TestPushCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()
	var publicVal atomic.Value // Written by the server

	// Fire up test server
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/uploads", func(w http.ResponseWriter, r *http.Request) {
			if m := r.Method; m != "POST" {
				t.Errorf("Incorrect method: %q", m)
			}

			err := r.ParseMultipartForm(1e6)
			if err != nil || r.MultipartForm == nil {
				t.Errorf("ParseMultipartForm err: %v", err)
				http.Error(w, "Bad form", http.StatusBadRequest)
				return
			}

			mf := r.MultipartForm
			if len(mf.File["file"]) == 0 {
				t.Errorf("No 'file' form field")
			}

			if vv := mf.Value["public"]; len(vv) != 0 {
				publicVal.Store(vv[0])
			} else {
				publicVal.Store("")
			}

			w.Write([]byte(pushResponse))
		})
	})

	cc := cli.TestContext(t.Context(), term, auth)
	flags := ctx.GlobalFlags(cc)
	flags.PushEndpoint = server.URL
	flags.Endpoint = server.URL

	packagePath := samplePackagePath()

	// Regular push without options
	err := runCommandNoErr(cc, []string{"push", packagePath})
	if err != nil {
		t.Fatal(err)
	}

	exp := fmt.Sprintf("Uploading %s - done", filepath.Base(packagePath))
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	} else if v := publicVal.Load(); v != "" {
		t.Errorf("Expected private, got %q", v)
	}

	// Regular push with "public"
	err = runCommandNoErr(cc, []string{"push", "--public", samplePackagePath()})
	if err != nil {
		t.Fatal(err)
	}

	exp = fmt.Sprintf("Uploading %s - done", filepath.Base(packagePath))
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	} else if v := publicVal.Load(); v != "true" {
		t.Errorf("Expected public, got %q", v)
	}
}

func TestPushCommandUnauthorized(t *testing.T) {
	server := testutil.APIServer(t, "POST", "/uploads", "[]", 200)
	args := []string{"push", samplePackagePath()}
	testCommandLoginPreCheck(t, args, server)
	server.Close()
}

func TestPushCommandForbidden(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	server := testutil.APIServer(t, "POST", "/uploads", "[]", 403)

	cc := testContext(t, term, auth, server)
	args := []string{"push", samplePackagePath()}
	if err := runCommand(cc, args); !errors.Is(err, api.ErrForbidden) {
		t.Errorf("Command not forbidden, error: %s", err)
	}

	// Status line on stdout, only the error on stderr
	expectOutput(t, term, "Uploading sample.txt - no permission\n", "Error: You're not allowed to do this\n")
}

// One missing file among two: the other is still uploaded, each file gets
// its status line on stdout, and stderr carries only the summary
func TestPushCommandPartialFailure(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()

	server := testutil.APIServer(t, "POST", "/uploads", pushResponse, 200)

	cc := testContext(t, term, auth, server)

	// One good file whose name contains a "%" (it must not be treated as a
	// format verb by the --quiet status line), and one missing file
	good := filepath.Join(t.TempDir(), "100%d.gem")
	if data, err := os.ReadFile(samplePackagePath()); err != nil {
		t.Fatal(err)
	} else if err := os.WriteFile(good, data, 0600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing.gem")

	err := runCommand(cc, []string{"push", "--quiet", good, missing})
	expectSummaryError(t, err, os.ErrNotExist, "1 of 2 uploads failed")

	expectOutput(t, term,
		"Uploading 100%d.gem - done\nUploading missing.gem - file not found\n",
		"Error: 1 of 2 uploads failed\n")
}

// A directory is refused before anything is uploaded
func TestPushCommandDirectory(t *testing.T) {
	for _, args := range [][]string{{"push"}, {"push", "--quiet"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			auth := terminal.TestAuther("user", "abc123", nil)
			term := terminal.NewForTest()

			server := offlineServer(t)
			dir := t.TempDir()

			cc := testContext(t, term, auth, server)
			err := runCommand(cc, append(args, dir))
			if !errors.Is(err, syscall.EISDIR) {
				t.Errorf("Expected syscall.EISDIR, got: %v", err)
			}

			expectOutput(t, term,
				fmt.Sprintf("Uploading %s - is a directory\n", filepath.Base(dir)),
				fmt.Sprintf("Error: read %s: is a directory\n", dir))
		})
	}
}

// Tests run in the directory of their package
func samplePackagePath() string {
	return filepath.Join("testdata", "sample.txt")
}
