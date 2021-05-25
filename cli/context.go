package cli

import (
	"context"
)

type contextKey int

const (
	ctxGlobalFlagsKey contextKey = iota
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
