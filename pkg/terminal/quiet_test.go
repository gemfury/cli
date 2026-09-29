package terminal

import (
	"testing"
)

// A result, a warning, and a question are shown with either flag.
// What is left out writes nothing, so Infof returns 0.
func TestQuiet(t *testing.T) {
	for name, tc := range map[string]struct {
		quiet, noProgress bool
		progress          int
		out               string
	}{
		"neither":               {false, false, 1, "info\nresult\n"},
		"--no-progress":         {false, true, 0, "info\nresult\n"},
		"--quiet":               {true, false, 0, "result\n"},
		"--quiet --no-progress": {true, true, 0, "result\n"},
	} {
		t.Run(name, func(t *testing.T) {
			user := NewForTest()
			user.SetPromptResponses(map[string]string{"Sure? [y/N]": "Y"})

			term := Quiet(user, tc.quiet, tc.noProgress)
			if n, _ := term.Infof("info\n"); (n == 0) != tc.quiet {
				t.Errorf("Expected nothing written if and only if --quiet, got %d bytes", n)
			}
			term.Printf("result\n")
			term.EPrintf("warning\n")
			term.StartProgress(1024, "Uploading x ").Finish()
			term.Spin(" Fetching ...")()

			if out := string(user.OutBytes()); out != tc.out {
				t.Errorf("Output should be %q, got %q", tc.out, out)
			}
			if errOut := string(user.ErrBytes()); errOut != "warning\n" {
				t.Errorf("Error output should be a warning, got %q", errOut)
			}
			if bars, spinners := user.ProgressShown(); bars != tc.progress || spinners != tc.progress {
				t.Errorf("Expected %d of each, got %d bars and %d spinners", tc.progress, bars, spinners)
			}
			if ok, err := term.Confirm("Sure? [y/N]"); !ok || err != nil {
				t.Errorf("Expected the user to be asked, got %v and: %v", ok, err)
			}
		})
	}
}
