package cli

import (
	"github.com/gemfury/cli/pkg/terminal"

	"context"
)

type contextKey int

const (
	ctxGlobalFlagsKey contextKey = iota
	ctxTerminalKey
)

type globalFlags struct {
	AuthToken string
	Account   string
}

func contextWithGlobalFlags(ctx context.Context) (*globalFlags, context.Context) {
	flags := &globalFlags{}
	ctx = context.WithValue(ctx, ctxGlobalFlagsKey, flags)
	return flags, ctx
}

func ctxGlobalFlags(ctx context.Context) *globalFlags {
	return ctx.Value(ctxGlobalFlagsKey).(*globalFlags)
}

func contextWithTerminal(ctx context.Context, t terminal.Terminal) context.Context {
	return context.WithValue(ctx, ctxTerminalKey, t)
}

func ctxTerminal(ctx context.Context) terminal.Terminal {
	return ctx.Value(ctxTerminalKey).(terminal.Terminal)
}
