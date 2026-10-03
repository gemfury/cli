package cli

import (
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"encoding/json"
)

// jsonCommand makes cmd one that prints its result as JSON with --json.
// The flag is its own, so any other command refuses it as unknown, lest
// its text be taken for JSON.
func jsonCommand(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Bool("json", false, "Print the result as JSON")
	return cmd
}

// printsJSON reports whether cmd was given --json. A command without
// the flag reports false.
func printsJSON(cmd *cobra.Command) bool {
	asJSON, _ := cmd.Flags().GetBool("json")
	return asJSON
}

// termPrintJSON prints v as one indented JSON document, without HTML
// escaping. The API structs give the shape: see the root help.
func termPrintJSON(term terminal.Terminal, v any) error {
	enc := json.NewEncoder(term.IOOut())
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
