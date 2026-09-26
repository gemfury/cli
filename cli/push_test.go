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
	"runtime"
	"strings"
	"testing"
)

const pushResponse = `{}`

// ==== push ====

func TestPushCommandSuccess(t *testing.T) {
	auth := terminal.TestAuther("user", "abc123", nil)
	term := terminal.NewForTest()
	var publicVal string

	// Fire up test server
	server := testutil.APIServerCustom(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/uploads", func(w http.ResponseWriter, r *http.Request) {
			if m := r.Method; m != "POST" {
				t.Errorf("Incorrect method: %q", m)
			}

			err := r.ParseMultipartForm(1e6)
			if err != nil || r.MultipartForm == nil {
				t.Fatalf("ParseMultipartForm err: %s", err)
			}

			mf := r.MultipartForm
			if len(mf.File["file"]) == 0 {
				t.Errorf("No 'file' form field")
			}

			if vv := mf.Value["public"]; len(vv) != 0 {
				publicVal = vv[0]
			} else {
				publicVal = ""
			}

			w.Write([]byte(pushResponse))
		})
	})

	defer server.Close()

	cc := cli.TestContext(term, auth)
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
	} else if publicVal != "" {
		t.Errorf("Expected private, got %q", publicVal)
	}

	// Regular push with "public"
	err = runCommandNoErr(cc, []string{"push", "--public", samplePackagePath()})
	if err != nil {
		t.Fatal(err)
	}

	exp = fmt.Sprintf("Uploading %s - done", filepath.Base(packagePath))
	if outStr := compactString(term.OutBytes()); !strings.HasSuffix(outStr, exp) {
		t.Errorf("Expected output to include %q, got %q", exp, outStr)
	} else if publicVal != "true" {
		t.Errorf("Expected public, got %q", publicVal)
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
	defer server.Close()

	cc := testContext(term, auth, server)
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
	defer server.Close()

	cc := testContext(term, auth, server)

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

func samplePackagePath() string {
	_, filename, _, _ := runtime.Caller(1)
	return filepath.Join(filename, "../testdata/sample.txt")
}
