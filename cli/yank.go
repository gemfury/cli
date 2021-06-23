package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"fmt"
)

// NewCmdYank generates the Cobra command for "yank"
func NewCmdYank() *cobra.Command {
	var versionFlag string

	yankCmd := &cobra.Command{
		Use:   "yank PACKAGE VERSION",
		Short: "Remove a package version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Please specify at least one package")
			}

			var pkg string = args[0]
			var ver string = ""

			if versionFlag != "" {
				ver = versionFlag
			} else if len(args) > 1 {
				ver = args[1]
			} else {
				return fmt.Errorf("No version specified")
			}

			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			err = c.Yank(cc, pkg, ver)
			if err == nil {
				term.Printf("Removed package %q version %q\n", pkg, ver)
			}

			return err
		},
	}

	// Flags and options
	yankCmd.Flags().StringVarP(&versionFlag, "version", "v", "", "Version")

	return yankCmd
}
