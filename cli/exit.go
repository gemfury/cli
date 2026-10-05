package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/pkg/terminal"

	"errors"
)

// Exit statuses of the command, as listed in the help of the root command
const (
	ExitOK          = 0
	ExitError       = 1 // Any failure without a status of its own
	ExitUsage       = 2 // Wrong arguments or flags, or confirmation needed
	ExitNotFound    = 3
	ExitAuth        = 4 // Not authenticated
	ExitExists      = 5 // Already exists, e.g. a version pushed again
	ExitUnavailable = 6 // Try again later
)

// ExitStatus is the exit status for the error of a command. A command that
// failed on several items exits with the status that all of those share,
// or else with ExitError.
func ExitStatus(err error) int {
	if err == nil {
		return ExitOK
	}

	// First, as any other check would match one item of several
	if multi := (*multiError)(nil); errors.As(err, &multi) {
		status := ExitStatus(multi.errs[0])
		for _, err := range multi.errs[1:] {
			if ExitStatus(err) != status {
				return ExitError
			}
		}
		return status
	}

	if IsUsageError(err) || errors.Is(err, terminal.ErrNoInput) {
		return ExitUsage
	}

	// An error of the API is told by way of UserError.Is
	switch {
	case errors.Is(err, api.ErrUnauthorized),
		errors.Is(err, ErrNotLoggedIn),
		errors.Is(err, ErrLoginUnattended),
		errors.Is(err, errLoginCancelled):
		return ExitAuth
	case errors.Is(err, api.ErrNotFound):
		return ExitNotFound
	case errors.Is(err, api.ErrAlreadyExists):
		return ExitExists
	case errors.Is(err, api.ErrConflict), isServerFailure(err):
		return ExitUnavailable
	}

	return ExitError
}

// isServerFailure reports whether err is about the server or the way to
// it, rather than about what was asked of it. Not every failed request
// is: an endpoint that is not a URL is not worth trying again.
func isServerFailure(err error) bool {
	return errors.Is(err, api.ErrTimeout) ||
		errors.Is(err, api.ErrFuryServer) ||
		errors.Is(err, api.ErrTooManyRequests) ||
		api.IsConnectionError(err)
}
