package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
)

// usageError is a mistake in command-line arguments or flags.
// The command's usage text is printed alongside these errors.
type usageError struct {
	msg string
	err error // Cause, if any
}

func (e *usageError) Error() string {
	return e.msg
}

func (e *usageError) Unwrap() error {
	return e.err
}

// usageErrorf creates a usageError with a formatted message
func usageErrorf(format string, a ...any) error {
	return &usageError{msg: fmt.Sprintf(format, a...)}
}

// asUsageError makes a usageError of err, which remains its cause
func asUsageError(err error) error {
	return &usageError{msg: err.Error(), err: err}
}

// IsUsageError reports whether err (or any error it wraps) is a usageError
func IsUsageError(err error) bool {
	var ue *usageError
	return errors.As(err, &ue)
}

// usageArgs wraps a Cobra argument check so that its failure is a usage
// error reading msg. Cobra runs these before authentication.
func usageArgs(check cobra.PositionalArgs, msg string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if check(cmd, args) != nil {
			return &usageError{msg: msg}
		}
		return nil
	}
}

// noArgs rejects any positional argument as a usage error
func noArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.NoArgs(cmd, args); err != nil {
		return asUsageError(err)
	}
	return nil
}

// groupCommand makes cmd a parent that only groups subcommands. Run by
// itself it shows its help, without authentication; an unknown subcommand
// is a usage error, as it is at the root.
func groupCommand(cmd *cobra.Command) *cobra.Command {
	cmd.Args = noArgs
	cmd.Annotations = skipAuth()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}
	return cmd
}

// aboutError is an error that names what it is about (see about)
type aboutError struct {
	kind, name string
	err        error
}

func (e *aboutError) Error() string {
	return fmt.Sprintf("%s %q: %s", e.kind, e.name, e.err)
}

func (e *aboutError) Unwrap() error {
	return e.err
}

// about names what the command asked for in its error, by kind:
// about("Package", "foo", err) reads `Package "foo": ...`.
//
// It is nil for no error, and the error itself for one that is about
// something else: the credentials, the server, a request that got no
// response, or a file, whose error names its path already.
func about(kind, name string, err error) error {
	var pathErr *fs.PathError
	var urlErr *url.Error

	switch {
	case err == nil,
		errors.As(err, &pathErr),
		errors.As(err, &urlErr),
		errors.Is(err, api.ErrUnauthorized),
		isServerFailure(err):
		return err
	}

	return &aboutError{kind: kind, name: name, err: err}
}

// noResults handles the end of a paginated listing that produced no items.
// It prints msg only when the listing was genuinely empty, not when it failed
// before returning anything, and reports whether the caller should stop.
func noResults(term terminal.Terminal, count int, err error, msg string) bool {
	if count > 0 {
		return false
	}
	if err == nil {
		term.Println(msg)
	}
	return true
}

// failures collects what went wrong in a command that processes several
// items (push, yank, sharing add, ...) and keeps going after one fails.
//
// With several items, each failure is reported on stderr as it happens
// and the command's error is a one-line summary, so stderr reads:
//
//	Problem adding "x@example.com": Doesn't look like this exists
//	Error: 1 of 3 invitations failed
//
// With a single item the failure itself is the command's error, about that
// item by its kind, and nothing extra is printed:
//
//	Error: Collaborator "x@example.com": Doesn't look like this exists
//
// A command that already reports each failed item itself uses record
// instead of add. As with errors.Join, a nil error is ignored, so a call's
// result can be passed straight in.
//
// An interrupted command stops at the item it was on and reports only the
// interruption, even if no item has failed. It is interrupted by a signal,
// or by the user at the prompt of an item.
type failures struct {
	cc    context.Context
	total int    // Items to attempt
	what  string // Names the items in the summary, e.g. "uploads"
	kind  string // Names an item in its failure, e.g. "File"
	errs  []error
}

// newFailures starts collecting for a command about to attempt total items
func newFailures(cc context.Context, total int, what, kind string) *failures {
	return &failures{cc: cc, total: total, what: what, kind: kind}
}

// interrupted reports whether to stop, leaving the remaining items unattempted
func (f *failures) interrupted() bool {
	return f.cc.Err() != nil || errors.Is(errors.Join(f.errs...), context.Canceled)
}

// add records a failed item, reporting it on stderr
// when there are several, unless the command is interrupted
func (f *failures) add(verb, item string, err error) {
	if err == nil {
		return
	}
	f.record(item, err)
	if f.total > 1 && !f.interrupted() {
		fmt.Fprintf(ctx.Terminal(f.cc).IOErr(), "Problem %s %q: %s\n", verb, item, err)
	}
}

// record counts the failure of an item without reporting it
func (f *failures) record(item string, err error) {
	if err != nil {
		f.errs = append(f.errs, about(f.kind, item, err))
	}
}

// err returns the command's error; a summary wraps every failure,
// so errors.Is and errors.As still see them
func (f *failures) err() error {
	switch {
	case f.interrupted():
		return context.Canceled
	case len(f.errs) == 0:
		return nil
	case f.total == 1:
		return f.errs[0]
	}
	return &multiError{
		summary: fmt.Sprintf("%d of %d %s failed", len(f.errs), f.total, f.what),
		errs:    f.errs,
	}
}

// multiError is the summary error for a partially failed multi-item command
type multiError struct {
	summary string
	errs    []error
}

func (e *multiError) Error() string {
	return e.summary
}

func (e *multiError) Unwrap() []error {
	return e.errs
}
