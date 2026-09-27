package ctx

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

// CmdGlobalFlags are the global flags as given on the command line. To get
// AuthToken or Account, use the cli helpers that fall back to environment,
// unless only the flag is of interest.
type CmdGlobalFlags struct {
	PushEndpoint string
	Endpoint     string
	AuthToken    string
	Account      string
}

func CmdContextWith(ctx context.Context, t terminal.Terminal, as terminal.Auther) context.Context {
	ctx = context.WithValue(ctx, ctxGlobalFlagsKey, &CmdGlobalFlags{})
	ctx = context.WithValue(ctx, ctxTerminalKey, t)
	ctx = context.WithValue(ctx, ctxAutherKey, as)
	return ctx
}

func GlobalFlags(ctx context.Context) *CmdGlobalFlags {
	return ctx.Value(ctxGlobalFlagsKey).(*CmdGlobalFlags)
}

func Auther(ctx context.Context) terminal.Auther {
	return ctx.Value(ctxAutherKey).(terminal.Auther)
}

func Terminal(ctx context.Context) terminal.Terminal {
	return ctx.Value(ctxTerminalKey).(terminal.Terminal)
}

func TestTerm(ctx context.Context) terminal.TestTerm {
	return ctx.Value(ctxTerminalKey).(terminal.TestTerm)
}
