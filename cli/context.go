package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"

	"context"
)

// CommandContext is the context for executing commands
// including global flags, auther, and terminal values
func CommandContext() context.Context {
	term, auth := terminal.New(), terminal.Netrc()
	return ctx.CmdContextWith(context.Background(), term, auth)
}

// TestContext is the context for executing commands in testing
func TestContext(t terminal.Terminal, a terminal.Auther) context.Context {
	return ctx.CmdContextWith(context.Background(), t, a)
}
