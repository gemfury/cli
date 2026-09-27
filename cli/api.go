package cli

import (
	"github.com/cenkalti/backoff/v7"
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"context"
	"errors"
	"os"
	"strings"
	"time"
)

// Initialize new Gemfury API client with authentication
func newAPIClient(cc context.Context) (*api.Client, error) {
	token, err := contextAuthToken(cc)
	if err != nil {
		return nil, err
	}

	return newAPIClientWithToken(cc, token), nil
}

// Initialize new Gemfury API client with an explicit token,
// bypassing the usual resolution by contextAuthToken
func newAPIClientWithToken(cc context.Context, token string) *api.Client {
	flags := ctx.GlobalFlags(cc)
	c := api.NewClient(token, contextAccount(cc))

	// Endpoint overrides (testing, staging). The client joins paths onto
	// these and compares URLs against them, so normalize away a trailing "/".
	if e := flags.PushEndpoint; e != "" {
		c.PushEndpoint = strings.TrimSuffix(e, "/")
	}
	if e := flags.Endpoint; e != "" {
		c.Endpoint = strings.TrimSuffix(e, "/")
	}

	return c
}

// Extract authentication token from context (inline or .netrc)
func contextAuthToken(cc context.Context) (string, error) {
	if token := inlineAuthToken(cc); token != "" {
		return token, nil
	}
	_, token, err := ctx.Auther(cc).Auth()
	return token, err
}

// inlineAuthToken is the token given for this invocation,
// rather than saved by "login": --api-token or else FURY_TOKEN
func inlineAuthToken(cc context.Context) string {
	return flagOrEnv(ctx.GlobalFlags(cc).AuthToken, "FURY_TOKEN")
}

// contextAccount is the account to act on: --account or else FURY_ACCOUNT.
// When empty, it is the account of the authenticated user.
func contextAccount(cc context.Context) string {
	return flagOrEnv(ctx.GlobalFlags(cc).Account, "FURY_ACCOUNT")
}

// flagOrEnv is the value of a global flag or, when the flag
// is not given, of the environment variable standing in for it
func flagOrEnv(flag, env string) string {
	if flag != "" {
		return flag
	}
	return strings.TrimSpace(os.Getenv(env))
}

// skipAuthAnnotation marks a command that must run without authentication,
// such as the ones that manage the session itself
const skipAuthKey = "fury.skip-auth"

var skipAuthAnnotation = map[string]string{skipAuthKey: "true"}

// skipsAuth reports whether cmd runs without authentication: commands
// annotated with skipAuthAnnotation, plus Cobra's built-in top-level
// help and shell-completion commands
func skipsAuth(cmd *cobra.Command) bool {
	if cmd.Annotations[skipAuthKey] == "true" {
		return true
	}

	if cmd.Parent() == cmd.Root() {
		switch cmd.Name() {
		case "help", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
			return true
		}
	}

	return false
}

// Hook for root command to ensure user is authenticated or prompt to login
func preRunCheckAuthentication(cmd *cobra.Command, args []string) error {
	if skipsAuth(cmd) {
		return nil
	}

	_, err := ensureAuthenticated(cmd, false)
	return err
}

// errLoginCancelled is returned when the user backs out of the login
// prompt. "login" exits quietly on it; other commands report it.
var errLoginCancelled = errors.New("Login cancelled")

// ErrNotLoggedIn is returned when there are no credentials and no user at
// the terminal to login (a pipe, CI, an agent), so login is not attempted
var ErrNotLoggedIn = errors.New(`Not logged in. Set FURY_TOKEN or run "fury login" in a terminal.`)

// ErrLoginUnattended is returned by "login" when there is no user to ask
// at the terminal, or none to be asked, whatever the saved credentials
var ErrLoginUnattended = errors.New("Cannot login with no one to ask. Set FURY_TOKEN to authenticate instead.")

func ensureAuthenticated(cmd *cobra.Command, interactive bool) (*api.AccountResponse, error) {
	cc := cmd.Context()
	var err error

	// Already authenticated with a token given inline or saved in .netrc
	if token, err := contextAuthToken(cc); token != "" || err != nil {
		return nil, err
	}

	// Login needs a user to answer its prompts. Without one,
	// fail before any request rather than start a login.
	if !ctx.Terminal(cc).IsInteractive() {
		return nil, ErrNotLoggedIn
	}

	// Trigger browser login
	var resp *api.LoginResponse
	if !interactive {
		resp, err = browserLogin(cmd)
		interactive = errors.Is(err, api.ErrNotImplemented)
	}

	// Trigger interactive login if requested by user
	// or if browser returned "not-implemented"
	if interactive {
		resp, err = interactiveLogin(cmd)
	}

	// Backing out of a login prompt, which includes Ctrl-C and Ctrl-D
	switch {
	case errors.Is(err, promptui.ErrAbort),
		errors.Is(err, promptui.ErrInterrupt),
		errors.Is(err, promptui.ErrEOF):
		return nil, errLoginCancelled
	case err != nil:
		return nil, err
	}

	// Save credentials to .netrc for future commands
	err = ctx.Auther(cc).Append(resp.User.Email, resp.Token)
	if err != nil {
		return nil, err
	}

	return &resp.User, nil
}

// loginPollTimeout is how long browserLogin waits for the user to approve
var loginPollTimeout = 3 * time.Minute

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
		return nil, errors.New("Internal error")
	}

	// Everything is ready. Confirm opening browser to login
	anyKey := "Press any key to login via the browser or q to exit: "
	if err := terminal.PromptAnyKeyOrQuit(term, anyKey); err != nil {
		return nil, err
	}

	// Attempt to open the browser to create CLI token
	term.Printf("Opening %s\n", createResp.BrowserURL)
	if ok := term.OpenBrowser(cc, createResp.BrowserURL); !ok {
		if err := cc.Err(); err != nil {
			return nil, err // Interrupted, rather than failed
		}
		term.Printf("Failed to open browser. You can continue CLI login by manually opening the URL\n")
	}

	// Start/end spinner while waiting for browser auth
	onDone := terminal.SpinIfTerminal(term, " Waiting ...")
	defer onDone()

	// LoginGet API will block & timeout, so we poll until a time limit,
	// which is shorter than the expiry of all the JWT tokens. The deadline
	// also ends a poll that is in flight, unlike backoff's own time limit.
	pollCtx, cancel := context.WithTimeout(cc, loginPollTimeout)
	defer cancel()

	resp, err := backoff.Retry(pollCtx, func() (*api.LoginGetResponse, error) {
		resp, err := c.LoginGet(pollCtx, createResp)
		if !errors.Is(err, api.ErrTimeout) && !errors.Is(err, api.ErrNotFound) {
			err = backoff.Permanent(err) // Retry only on timeout or not-found
		}
		return resp, err
	},
		// We retry with constant backoff waiting for user to approve login
		backoff.WithBackOff(backoff.NewConstantBackOff(500*time.Millisecond)),
		backoff.WithMaxElapsedTime(0), // Limited by pollCtx
	)

	// Report why the login failed, rather than why polling stopped
	if re := backoff.AsRetryError(err); re != nil {
		switch {
		case cc.Err() != nil:
			err = cc.Err() // Interrupted
		case pollCtx.Err() != nil || errors.Is(re.LastErr, api.ErrNotFound):
			err = api.ErrTimeout
		default:
			err = re.LastErr
		}
	}

	if resp == nil {
		return nil, err
	}

	if resp.Error != "" {
		err = errors.New(resp.Error)
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
	return c.Login(cc, &req)
}
