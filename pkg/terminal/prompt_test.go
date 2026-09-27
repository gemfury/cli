package terminal

import (
	"github.com/manifoldco/promptui"

	"regexp"
	"strings"
	"testing"
	"text/template"
)

// Escape sequences that style the text of a terminal
var stylingRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// A "y/N" question is shown as it is, without PromptUI adding to it:
// while it is asked, and once it is declined or answered
func TestConfirmPromptTemplates(t *testing.T) {
	const label = "Are you sure you want to logout? [y/N]"
	declined := stylingRegexp.ReplaceAllString(promptui.IconBad, "")

	p := confirmPrompt(label)
	if !p.IsConfirm {
		t.Errorf("Expected a confirmation prompt")
	}

	for name, tc := range map[string]struct {
		template string
		shown    string // Without its styling, and up to the answer
	}{
		"Confirm": {p.Templates.Confirm, "? " + label + " "},
		"Invalid": {p.Templates.Invalid, declined + " " + label + " "},
		"Success": {p.Templates.Success, label + " "},
	} {
		t.Run(name, func(t *testing.T) {
			tmpl, err := template.New("").Funcs(promptui.FuncMap).Parse(tc.template)
			if err != nil {
				t.Fatal(err)
			}

			var out strings.Builder
			if err := tmpl.Execute(&out, p.Label); err != nil {
				t.Fatal(err)
			}

			if shown := stylingRegexp.ReplaceAllString(out.String(), ""); shown != tc.shown {
				t.Errorf("Expected %q to be shown, got %q", tc.shown, shown)
			}
		})
	}
}
