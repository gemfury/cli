package cli

import (
	"github.com/gemfury/cli/pkg/terminal"

	"context"
)

type contextKey int

const (
	ctxGlobalFlagsKey contextKey = iota
	ctxTerminalKey
	ctxAutherKey
)

type globalFlags struct {
	Endpoint  string
	AuthToken string
	Account   string
}

func cmdContextWith(ctx context.Context, t terminal.Terminal, as terminal.Auther) context.Context {
	ctx = context.WithValue(ctx, ctxGlobalFlagsKey, &globalFlags{})
	ctx = context.WithValue(ctx, ctxTerminalKey, t)
	ctx = context.WithValue(ctx, ctxAutherKey, as)
	return ctx
}

func ContextGlobalFlags(ctx context.Context) *globalFlags {
	return ctx.Value(ctxGlobalFlagsKey).(*globalFlags)
}

func ctxTerminal(ctx context.Context) terminal.Terminal {
	return ctx.Value(ctxTerminalKey).(terminal.Terminal)
}

func ctxAuther(ctx context.Context) terminal.Auther {
	return ctx.Value(ctxAutherKey).(terminal.Auther)
}

// CommandContext is the context for executing commands
// including global flags, auther, and terminal values
func CommandContext() context.Context {
	term, auth := terminal.New(), terminal.Netrc()
	return cmdContextWith(context.Background(), term, auth)
}

// TestContext is the context for executing commands in testing
func TestContext(t terminal.Terminal, a terminal.Auther) context.Context {
	return cmdContextWith(context.Background(), t, a)
}
