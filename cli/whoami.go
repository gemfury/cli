package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"context"
)

// NewCmdWhoAmI generates the Cobra command for "whoami"
func NewCmdWhoAmI() *cobra.Command {
	whoCmd := jsonCommand(&cobra.Command{
		Use:   "whoami",
		Short: "Show current account",
		Example: `  fury whoami
  FURY_TOKEN=token fury whoami`,
		Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			resp, err := whoAMI(cc)
			if err != nil {
				return err
			}

			term := ctx.Terminal(cc)
			if printsJSON(cmd) {
				return termPrintJSON(term, resp)
			}

			term.Printf("You are logged in as %q\n", resp.Name)
			return nil
		},
	})

	return whoCmd
}

func whoAMI(cc context.Context) (*api.AccountResponse, error) {
	c, err := newAPIClient(cc)
	if err != nil {
		return nil, err
	}

	return c.WhoAmI(cc)
}
