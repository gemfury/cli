package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"

	"context"
)

// CommandContext is the context for executing commands
// including global flags, auther, and terminal values
func CommandContext() context.Context {
	term := terminal.New()
	auth := terminal.CredentialStore(term)
	return ctx.CmdContextWith(context.Background(), term, auth)
}

// TestContext is the context for executing commands in testing,
// derived from the parent context, such as the one of the test
func TestContext(parent context.Context, t terminal.Terminal, a terminal.Auther) context.Context {
	return ctx.CmdContextWith(parent, t, a)
}
