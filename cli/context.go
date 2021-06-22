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

func contextWithTerminal(ctx context.Context, t terminal.Terminal, as terminal.Auther) context.Context {
	ctx = context.WithValue(ctx, ctxTerminalKey, t)
	ctx = context.WithValue(ctx, ctxAutherKey, as)
	return ctx
}

func ctxTerminal(ctx context.Context) terminal.Terminal {
	return ctx.Value(ctxTerminalKey).(terminal.Terminal)
}

func ctxAuther(ctx context.Context) terminal.Auther {
	return ctx.Value(ctxAutherKey).(terminal.Auther)
}
