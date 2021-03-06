package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"fmt"
	"log"
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
	term := ctx.Terminal(cc)
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
		term.Println("No members found for this account")
		return err
	}

	// Print results
	term.Printf("*** Collaborators ***\n")
	w := tabwriter.NewWriter(term.IOOut(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "name\trole\n")

	for _, m := range members {
		fmt.Fprintf(w, "%s\t%s\n", m.Name, m.Role)
	}

	w.Flush()
	return err
}

// NewCmdSharingAdd generates the Cobra command for "sharing:add"
func NewCmdSharingAdd() *cobra.Command {
	var roleFlag string

	addCmd := &cobra.Command{
		Use:   "add EMAIL",
		Short: "Add a collaborator",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Please specify at least one collaborator")
			}

			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			var multiErr *multierror.Error
			for _, name := range args {
				err := c.AddCollaborator(cc, name, roleFlag)

				if err != nil {
					multiErr = multierror.Append(multiErr, err)
					log.Printf("Problem adding %q: %s\n", name, err)
					continue
				}

				term.Printf("Invited %q as a collaborator\n", name)
			}

			return multiErr.Unwrap()
		},
	}

	// Flags and options
	addCmd.Flags().StringVar(&roleFlag, "role", "", "Collaborator role")

	return addCmd
}

// NewCmdSharingRemove generates the Cobra command for "sharing:add"
func NewCmdSharingRemove() *cobra.Command {
	rmCmd := &cobra.Command{
		Use:   "remove EMAIL",
		Short: "Remove a collaborator",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Please specify at least one collaborator")
			}

			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			var multiErr *multierror.Error
			for _, name := range args {
				err := c.RemoveCollaborator(cc, name)

				if err != nil {
					multiErr = multierror.Append(multiErr, err)
					log.Printf("Problem removing %q: %s\n", name, err)
					continue
				}

				term.Printf("Removed %q as a collaborator\n", name)
			}

			return multiErr.Unwrap()
		},
	}

	return rmCmd
}

// Root for sharing/collaboration subcommands
func NewCmdAccounts() *cobra.Command {
	accountsCmd := &cobra.Command{
		Use:   "accounts",
		Short: "Listing of your collaborations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			members := []*api.Member{}

			// Paginate over package listings until no more pages
			err = iterateAllPages(cc, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
				resp, err := c.Collaborations(cc, pageReq)
				if err != nil {
					return nil, err
				}

				members = append(members, resp.Members...)
				return resp.Pagination, nil
			})

			// Handle no packages
			if len(members) == 0 {
				term.Println("No collaborations found for this account")
				return err
			}

			// Print results
			w := tabwriter.NewWriter(term.IOOut(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "name\tkind\trole\n")

			for _, m := range members {
				fmt.Fprintf(w, "%s\t%s\t%s\n", m.Name, m.Type, m.Role)
			}

			w.Flush()
			return err
		},
	}

	return accountsCmd
}
