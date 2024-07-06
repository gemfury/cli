package cli

import (
	"github.com/cenkalti/backoff/v4"
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"context"
	"errors"
	"fmt"
	"time"
)

// Initialize new Gemfury API client with authentication
func newAPIClient(cc context.Context) (c *api.Client, err error) {
	flags := ctx.GlobalFlags(cc)

	// Token comes from CLI flags or .netrc
	token, err := contextAuthToken(cc)
	if err != nil {
		return nil, err
	}

	// Initialize client with authentication
	c = api.NewClient(token, flags.Account)

	// Endpoint configuration for testing
	if e := flags.PushEndpoint; e != "" {
		c.PushEndpoint = e
	}
	if e := flags.Endpoint; e != "" {
		c.Endpoint = e
	}

	return c, nil
}

// Extract authentication token from context (flag or .netrc)
func contextAuthToken(cc context.Context) (string, error) {
	if token := ctx.GlobalFlags(cc).AuthToken; token != "" {
		return token, nil
	}
	_, token, err := ctx.Auther(cc).Auth()
	return token, err
}

// Hook for root command to ensure user is authenticated or prompt to login
func preRunCheckAuthentication(cmd *cobra.Command, args []string) error {
	if n := cmd.Name(); n == "logout" || n == "login" {
		return nil
	}

	_, err := ensureAuthenticated(cmd, false)
	return err
}

func ensureAuthenticated(cmd *cobra.Command, interactive bool) (*api.AccountResponse, error) {
	cc := cmd.Context()
	var err error

	// Check whether we have login credentials from environment
	if token, err := contextAuthToken(cc); token != "" || err != nil {
		return nil, err
	}

	// Browser or interactive login
	var resp *api.LoginResponse
	if interactive {
		resp, err = interactiveLogin(cmd)
	} else {
		resp, err = browserLogin(cmd)
	}
	if err != nil {
		return nil, err
	}

	// Save credentials to .netrc for future commands
	err = ctx.Auther(cc).Append(resp.User.Email, resp.Token)
	if err != nil {
		return nil, err
	}

	return &resp.User, nil
}

// browserLogin is a challenge/response authentication via browser
func browserLogin(cmd *cobra.Command) (*api.LoginResponse, error) {
	cc := cmd.Context()
	term := ctx.Terminal(cc)

	c, err := newAPIClient(cc)
	if err != nil {
		return nil, err
	}

	// Generate authentication URLs
	createResp, err := c.LoginCreate(cc)
	if err != nil {
		return nil, err
	} else if createResp.BrowserURL == "" {
		return nil, fmt.Errorf("Internal error")
	}

	// Everything is ready. Confirm opening browser to login
	anyKey := "Press any key to login via the browser or q to exit: "
	if err := terminal.PromptAnyKeyOrQuit(term, anyKey); err != nil {
		return nil, err
	}

	// Attempt to open the browser to create CLI token
	term.Printf("Opening %s\n", createResp.BrowserURL)
	if ok := term.OpenBrowser(createResp.BrowserURL); !ok {
		term.Printf("Failed to open browser. You can continue CLI login by manually opening the URL\n")
	}

	// Start/end spinner while waiting for browser auth
	onDone := terminal.SpinIfTerminal(term, " Waiting ...")
	defer onDone()

	// LoginGet will timeout, so we retry until a time limit.
	// We do constant backoff with elapsed time limit that
	// is shorter than the expiry of all the JWT tokens.
	constantBackoff := backoff.NewExponentialBackOff(
		backoff.WithMaxElapsedTime(3*time.Minute),
		backoff.WithMultiplier(1.0),
	)

	// Repeatedly hit LoginGet API until results
	var resp *api.LoginGetResponse
	err = backoff.Retry(func() error {
		resp, err = c.LoginGet(cc, createResp)
		if !errors.Is(err, api.ErrTimeout) && !errors.Is(err, api.ErrNotFound) {
			err = backoff.Permanent(err) // Retry only on timeout or not-found
		}
		return err
	}, backoff.WithContext(constantBackoff, cc))

	if resp == nil {
		return nil, err
	}

	if resp.Error != "" {
		err = fmt.Errorf(resp.Error)
	} else if errors.Is(err, api.ErrNotFound) {
		err = api.ErrTimeout
	}

	return &resp.LoginResponse, err
}

// interactiveLogin is an email/password authentication via terminal
func interactiveLogin(cmd *cobra.Command) (*api.LoginResponse, error) {
	cc := cmd.Context()

	// Interactive login
	term := ctx.Terminal(cc)
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
	}
	return resp, err
}
