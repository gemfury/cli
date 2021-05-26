package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

// NewCmdWhoAmI generates the Cobra command for "whoami"
func NewCmdWhoAmI() *cobra.Command {
	whoCmd := &cobra.Command{
		Use:   "whoami",
		Short: "Return current account",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}

			resp, err := c.WhoAmI(cmd.Context())
			if err != nil {
				return err
			}

			fmt.Printf("You are logged in as %q\n", resp.Name)
			return nil
		},
	}

	return whoCmd
}
