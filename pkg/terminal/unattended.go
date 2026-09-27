package terminal

import (
	"errors"
)

// ErrNoInput is returned for a question that there is no one to answer
var ErrNoInput = errors.New("Confirmation needed. Pass --yes to confirm, as there is no one to ask.")

// unattended is a Terminal that answers for the user, or says that it cannot
type unattended struct {
	Terminal
	yes     bool // Every "y/N" question is answered yes
	noInput bool // No question is asked
}

// Unattended wraps a Terminal to answer its questions as the --yes
// and --no-input flags ask. Without either flag, questions are asked
// as before, unless there is no user at the Terminal to answer them.
func Unattended(t Terminal, yes, noInput bool) Terminal {
	return unattended{Terminal: t, yes: yes, noInput: noInput}
}

func (u unattended) IsInteractive() bool {
	return !u.noInput && u.Terminal.IsInteractive()
}

func (u unattended) Confirm(label string) (bool, error) {
	switch {
	case u.yes:
		return true, nil
	case !u.IsInteractive():
		return false, ErrNoInput
	}
	return u.Terminal.Confirm(label)
}
