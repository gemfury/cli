package cli

import (
	"github.com/spf13/cobra"

	"fmt"
	"os"
	"path/filepath"
)

// NewCmdPush generates the Cobra command for "push"
func NewCmdPush() *cobra.Command {
	pushCmd := &cobra.Command{
		Use:   "push PACKAGE",
		Short: "Upload a new version of a package",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Please specify at least one package")
			}

			cc := cmd.Context()
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			for _, path := range args {
				err := func() error {
					file, err := os.Open(path)
					if err != nil {
						return err
					}
					defer file.Close()

					name := filepath.Base(file.Name())
					err = c.PushPkg(cc, name, file)
					if err != nil {
						return err
					}

					fmt.Printf("Uploaded package %q\n", name)
					return nil
				}()

				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	return pushCmd
}
