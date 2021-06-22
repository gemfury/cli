package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// Hook for root command to ensure user is authenticated or prompt to login
func preRunCheckAuthentication(cmd *cobra.Command, args []string) error {
	if n := cmd.Name(); n == "logout" {
		return nil
	}

	_, err := ensureAuthenticated(cmd)
	return err
}

func ensureAuthenticated(cmd *cobra.Command) (*api.AccountResponse, error) {
	if token, err := netrcAuth(); token != "" || err != nil {
		return nil, err
	}

	term := ctxTerminal(cmd.Context())
	term.Println("Please enter your Gemfury credentials.")

	ePrompt := promptui.Prompt{Label: "Email: "}
	eResult, err := term.RunPrompt(&ePrompt)
	if err != nil {
		return nil, err
	}

	pPrompt := promptui.Prompt{Label: "Password: ", Mask: '*'}
	pResult, err := term.RunPrompt(&pPrompt)
	if err != nil {
		return nil, err
	}

	c, err := newAPIClient(cmd.Context())
	if err != nil {
		return nil, err
	}

	req := api.LoginRequest{Email: eResult, Password: pResult}
	resp, err := c.Login(cmd.Context(), &req)
	if err == api.ErrUnauthorized {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return nil, err
	} else if err != nil {
		return nil, err
	}

	// Save credentials in .netrc
	err = netrcAppend(netrcMachines, resp.User.Email, resp.Token)
	if err != nil {
		return nil, err
	}

	return &resp.User, nil
}
