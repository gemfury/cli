package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"fmt"
	"strings"
)

// NewCmdYank generates the Cobra command for "yank"
func NewCmdYank() *cobra.Command {
	var versionFlag string

	yankCmd := &cobra.Command{
		Use:   "yank PACKAGE@VERSION",
		Short: "Remove a package version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Please specify at least one package")
			}

			if versionFlag != "" && len(args) > 1 {
				return fmt.Errorf("Use PACKAGE@VERSION for multiple yanks")
			}

			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			var multiErr *multierror.Error
			for _, pkg := range args {
				var ver string = ""

				if versionFlag != "" {
					ver = versionFlag
				} else if at := strings.LastIndex(pkg, "@"); at > 0 {
					pkg, ver = pkg[0:at], pkg[at+1:]
				}

				if pkg == "" || ver == "" {
					err := fmt.Errorf("Invalid package/version specified")
					multiErr = multierror.Append(multiErr, err)
					continue
				}

				err = c.Yank(cc, pkg, ver)
				if err != nil {
					multiErr = multierror.Append(multiErr, err)
					continue
				}

				term.Printf("Removed package %q version %q\n", pkg, ver)
			}

			return multiErr.Unwrap()
		},
	}

	// Flags and options
	yankCmd.Flags().StringVarP(&versionFlag, "version", "v", "", "Version")

	return yankCmd
}
