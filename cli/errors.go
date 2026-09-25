package cli

import (
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"errors"
	"fmt"
)

// usageError is a mistake in command-line arguments or flags.
// The command's usage text is printed alongside these errors.
type usageError struct {
	msg string
}

func (e *usageError) Error() string {
	return e.msg
}

// usageErrorf creates a usageError with a formatted message
func usageErrorf(format string, a ...interface{}) error {
	return &usageError{msg: fmt.Sprintf(format, a...)}
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
		return usageErrorf("%s", err)
	}
	return nil
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
// With a single item the failure itself is the command's error and nothing
// extra is printed. A command that already reports each failed item itself
// uses record instead of add. As with errors.Join, a nil error is ignored,
// so a call's result can be passed straight in.
type failures struct {
	term  terminal.Terminal
	total int    // Items attempted
	what  string // Names the items in the summary, e.g. "uploads"
	errs  []error
}

// newFailures starts collecting for a command about to attempt total items
func newFailures(term terminal.Terminal, total int, what string) *failures {
	return &failures{term: term, total: total, what: what}
}

// add records a failed item, reporting it on stderr when there are several
func (f *failures) add(verb, item string, err error) {
	if err == nil {
		return
	}
	if f.total > 1 {
		fmt.Fprintf(f.term.IOErr(), "Problem %s %q: %s\n", verb, item, err)
	}
	f.record(err)
}

// record counts a failure without reporting it
func (f *failures) record(err error) {
	if err != nil {
		f.errs = append(f.errs, err)
	}
}

// err returns the command's error; a summary wraps every failure,
// so errors.Is and errors.As still see them
func (f *failures) err() error {
	switch {
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
