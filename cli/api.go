package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"context"
)

// Initialize new Gemfury API client with authentication
func newAPIClient(cc context.Context) (c *api.Client, err error) {
	flags := ctxGlobalFlags(cc)

	// Token comes from CLI flags or .netrc
	token := flags.AuthToken
	if token == "" {
		_, token, err = ctxAuther(cc).Auth()
		if err != nil {
			return nil, err
		}
	}

	c = api.NewClient(token, flags.Account)
	return c, nil
}

// Hook for root command to ensure user is authenticated or prompt to login
func preRunCheckAuthentication(cmd *cobra.Command, args []string) error {
	if n := cmd.Name(); n == "logout" {
		return nil
	}

	_, err := ensureAuthenticated(cmd)
	return err
}

func ensureAuthenticated(cmd *cobra.Command) (*api.AccountResponse, error) {
	cc := cmd.Context()

	if _, token, err := ctxAuther(cc).Auth(); token != "" || err != nil {
		return nil, err
	}

	term := ctxTerminal(cc)
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

	c, err := newAPIClient(cc)
	if err != nil {
		return nil, err
	}

	req := api.LoginRequest{Email: eResult, Password: pResult}
	resp, err := c.Login(cc, &req)
	if err == api.ErrUnauthorized {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return nil, err
	} else if err != nil {
		return nil, err
	}

	// Save credentials in .netrc
	err = ctxAuther(cc).Append(resp.User.Email, resp.Token)
	if err != nil {
		return nil, err
	}

	return &resp.User, nil
}
