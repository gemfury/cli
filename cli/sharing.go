package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/spf13/cobra"

	"fmt"
	"log"
	"os"
	"text/tabwriter"
)

// Root for sharing/collaboration subcommands
func NewCmdSharingRoot() *cobra.Command {
	gitCmd := &cobra.Command{
		Use:   "sharing",
		Short: "Collaboration commands",
		RunE:  listMembers,
	}

	gitCmd.AddCommand(NewCmdSharingAdd())
	gitCmd.AddCommand(NewCmdSharingRemove())

	return gitCmd
}

func listMembers(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	members := []*api.Member{}

	// Paginate over package listings until no more pages
	err = iterateAllPages(cc, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
		resp, err := c.Members(cc, pageReq)
		if err != nil {
			return nil, err
		}

		members = append(members, resp.Members...)
		return resp.Pagination, nil
	})

	// Handle no packages
	if len(members) == 0 {
		fmt.Println("No members found for this account")
		return nil
	}

	// Print results
	fmt.Printf("*** Collaborators ***\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "name\trole\n")

	for _, m := range members {
		fmt.Fprintf(w, "%s\t%s\n", m.Name, m.Role)
	}

	w.Flush()
	return nil
}

// NewCmdSharingAdd generates the Cobra command for "sharing:add"
func NewCmdSharingAdd() *cobra.Command {
	addCmd := &cobra.Command{
		Use:   "add EMAIL",
		Short: "Add a collaborator",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Please specify at least one collaborator")
			}

			cc := cmd.Context()
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			for _, name := range args {
				err := c.AddCollaborator(cc, name)

				if err != nil {
					log.Printf("Problem adding %q: %s\n", name, err)
					continue
				}

				fmt.Printf("Invited %q as a collaborator\n", name)
			}

			return nil
		},
	}

	return addCmd
}

// NewCmdSharingRemove generates the Cobra command for "sharing:add"
func NewCmdSharingRemove() *cobra.Command {
	addCmd := &cobra.Command{
		Use:   "remove EMAIL",
		Short: "Remove a collaborator",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Please specify at least one collaborator")
			}

			cc := cmd.Context()
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			for _, name := range args {
				err := c.RemoveCollaborator(cc, name)

				if err != nil {
					log.Printf("Problem removing %q: %s\n", name, err)
					continue
				}

				fmt.Printf("Removed %q as a collaborator\n", name)
			}

			return nil
		},
	}

	return addCmd
}
