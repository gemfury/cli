package cli

import (
	"github.com/spf13/cobra"

	"fmt"
	"log"
)

func NewCmdWhoAmI() *cobra.Command {
	whoCmd := &cobra.Command{
		Use:   "whoami",
		Short: "Return current account",
		Run: func(cmd *cobra.Command, args []string) {
			c, err := newAPIClient(cmd.Context())
			if err != nil {
				log.Fatal(err)
			}

			resp, err := c.WhoAmI(cmd.Context())
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("You are logged in as %q\n", resp.Name)
		},
	}

	return whoCmd
}
