package terminal

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// A progress bar must never render to a non-terminal error stream,
// whether it has a file descriptor (pipe) or not (buffer)
func TestStartProgressNonTerminal(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()

	var buf bytes.Buffer
	streams := map[string]io.WriteCloser{
		"pipe":   pw,
		"buffer": writeCloser{&buf},
	}

	for name, w := range streams {
		p := term{ioErr: w}.StartProgress(1024, "Uploading x ")

		// Reading through the proxy must be a pass-through
		body, _ := io.ReadAll(p.NewProxyReader(strings.NewReader("hello")))
		if string(body) != "hello" {
			t.Errorf("%s: proxy reader altered data: %q", name, body)
		}
		p.Finish()
	}

	pw.Close()
	if out, _ := io.ReadAll(pr); len(out) != 0 {
		t.Errorf("pipe received progress output: %q", out)
	}
	if buf.Len() != 0 {
		t.Errorf("buffer received progress output: %q", buf.Bytes())
	}
}
