package terminal

import (
	"github.com/manifoldco/promptui"

	"errors"
	"testing"
)

// A "y/N" question is answered by --yes, or else by the user, if there
// is one to ask. No other question is answered for the user.
func TestUnattended(t *testing.T) {
	for name, tc := range map[string]struct {
		yes, noInput bool
		user         string // Answer of the one at the terminal, if any
		confirmed    bool
		err          error
	}{
		"asked, and confirmed": {false, false, "Y", true, nil},
		"asked, and declined":  {false, false, "N", false, nil},
		"no one to ask":        {false, false, "", false, ErrNoInput},
		"--no-input":           {false, true, "Y", false, ErrNoInput},
		"--yes":                {true, false, "N", true, nil},
		"--yes, no one to ask": {true, false, "", true, nil},
		"--yes --no-input":     {true, true, "N", true, nil},
	} {
		t.Run(name, func(t *testing.T) {
			user := NewForTest()
			user.SetInteractive(tc.user != "")
			user.SetPromptResponses(map[string]string{"Sure? [y/N]": tc.user, "Name: ": "typed"})

			term := Unattended(user, tc.yes, tc.noInput)
			if exp := tc.user != "" && !tc.noInput; term.IsInteractive() != exp {
				t.Errorf("Expected interactive=%v", exp)
			}

			confirmed, err := term.Confirm("Sure? [y/N]")
			if confirmed != tc.confirmed || !errors.Is(err, tc.err) {
				t.Errorf("Expected confirmed=%v and error %v, got %v and: %v", tc.confirmed, tc.err, confirmed, err)
			}

			if name, err := term.RunPrompt(&promptui.Prompt{Label: "Name: "}); name != "typed" || err != nil {
				t.Errorf("Expected the user to be asked, got %q and: %v", name, err)
			}
		})
	}
}
