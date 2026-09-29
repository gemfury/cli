package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"fmt"
	"text/tabwriter"
)

// collaboratorArgs is the Args check for commands taking EMAIL...
var collaboratorArgs = usageArgs(cobra.MinimumNArgs(1), "Please specify at least one collaborator")

// Root for sharing/collaboration subcommands
func NewCmdSharingRoot() *cobra.Command {
	sharingCmd := &cobra.Command{
		Use:   "sharing",
		Short: "Collaboration commands",
		Long:  `List the collaborators of this account, or add and remove them.`,
		Example: `  fury sharing
  fury sharing --account my-org`,
		Args: noArgs,
		RunE: listMembers,
	}

	sharingCmd.AddCommand(NewCmdSharingAdd())
	sharingCmd.AddCommand(NewCmdSharingRemove())

	return sharingCmd
}

func listMembers(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	members := []*api.Member{}

	// Paginate over member listings until no more pages
	err = iterateAllPages(cc, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
		resp, err := c.Members(cc, pageReq)
		if err != nil {
			return nil, err
		}

		members = append(members, resp.Members...)
		return resp.Pagination, nil
	})

	if noResults(term, len(members), err, "No members found for this account") {
		return err
	}

	// Print results
	term.Infof("*** Collaborators ***\n")
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
		Use:   "add EMAIL...",
		Short: "Add a collaborator",
		Example: `  fury sharing add dev@example.com
  fury sharing add dev@example.com ops@example.com --role push`,
		Args: collaboratorArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			fails := newFailures(cc, len(args), "invitations", "Collaborator")
			for _, name := range args {
				if fails.interrupted() {
					break
				}

				if err := c.AddCollaborator(cc, name, roleFlag); err != nil {
					fails.add("adding", name, err)
					continue
				}

				term.Infof("Invited %q as a collaborator\n", name)
			}

			return fails.err()
		},
	}

	// Flags and options
	addCmd.Flags().StringVar(&roleFlag, "role", "", "Collaborator role: pull, push, or owner")

	return addCmd
}

// NewCmdSharingRemove generates the Cobra command for "sharing:remove"
func NewCmdSharingRemove() *cobra.Command {
	rmCmd := &cobra.Command{
		Use:     "remove EMAIL...",
		Short:   "Remove a collaborator",
		Example: `  fury sharing remove dev@example.com`,
		Args:    collaboratorArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			fails := newFailures(cc, len(args), "removals", "Collaborator")
			for _, name := range args {
				if fails.interrupted() {
					break
				}

				if err := c.RemoveCollaborator(cc, name); err != nil {
					fails.add("removing", name, err)
					continue
				}

				term.Infof("Removed %q as a collaborator\n", name)
			}

			return fails.err()
		},
	}

	return rmCmd
}

// NewCmdAccounts lists the accounts that the user collaborates on
func NewCmdAccounts() *cobra.Command {
	accountsCmd := &cobra.Command{
		Use:   "accounts",
		Short: "Listing of your collaborations",
		Long: `List the accounts that you collaborate on, each of which
can be given to --account.`,
		Example: `  fury accounts`,
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			members := []*api.Member{}

			// Paginate over collaboration listings until no more pages
			err = iterateAllPages(cc, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
				resp, err := c.Collaborations(cc, pageReq)
				if err != nil {
					return nil, err
				}

				members = append(members, resp.Members...)
				return resp.Pagination, nil
			})

			if noResults(term, len(members), err, "No collaborations found for this account") {
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
