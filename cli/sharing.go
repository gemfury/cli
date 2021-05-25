package cli

import (
	"github.com/spf13/cobra"

	"fmt"
	"log"
)

func NewCmdSharingAdd() *cobra.Command {
	addCmd := &cobra.Command{
		Use:   "sharing:add",
		Short: "Invite an account to collaborate",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				log.Fatal("Please specify at least one collaborator")
			}

			cc := cmd.Context()
			c, err := newAPIClient(cc)
			if err != nil {
				log.Fatal(err)
			}

			for _, name := range args {
				err := c.AddCollaborator(cc, name)

				if err != nil {
					log.Printf("Problem adding %q: %s\n", name, err)
					continue
				}

				fmt.Printf("Invited %q as a collaborator\n", name)
			}
		},
	}

	return addCmd
}
