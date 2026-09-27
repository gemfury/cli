package terminal

import (
	"github.com/briandowns/spinner"
	"github.com/manifoldco/promptui"
	xterm "golang.org/x/term"

	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// PromptConfirm asks a "y/N" question from Stdin. Leaving it unanswered,
// with Ctrl-C or Ctrl-D, interrupts the command rather than declines.
func PromptConfirm(t Terminal, label string) (bool, error) {
	_, err := t.RunPrompt(confirmPrompt(label))
	switch {
	case errors.Is(err, promptui.ErrAbort):
		return false, nil
	case errors.Is(err, promptui.ErrInterrupt), errors.Is(err, promptui.ErrEOF):
		return false, context.Canceled
	}
	return err == nil, err
}

// confirmTemplates show the label as it is, for it ends in "? [y/N]" already.
// PromptUI would otherwise add the same to the question being asked (Confirm),
// and a colon once that is declined (Invalid) or answered (Success).
var confirmTemplates = promptui.PromptTemplates{
	Confirm: fmt.Sprintf(iconLabelTemplate, promptui.IconInitial),
	Invalid: fmt.Sprintf(iconLabelTemplate, promptui.IconBad),
	Success: `{{ . | faint }} `,
}

const iconLabelTemplate = `{{ "%s" | bold }} {{ . | bold }} `

// confirmPrompt is the "y/N" question that PromptConfirm asks
func confirmPrompt(label string) *promptui.Prompt {
	templates := confirmTemplates // PromptUI prepares the templates it is given
	return &promptui.Prompt{Label: label, IsConfirm: true, Templates: &templates}
}

// Control bytes that mean "quit" at a single-key prompt. The terminal is in
// raw mode while reading, so Ctrl-C arrives as a byte rather than a signal.
const (
	keyCtrlC = 0x03
	keyCtrlD = 0x04
	keyEsc   = 0x1b
)

// PromptAnyKeyOrQuit reads a single key from Stdin. It returns
// promptui.ErrAbort when the user backs out with "q", Esc, Ctrl-C, or Ctrl-D.
func PromptAnyKeyOrQuit(t Terminal, prompt string) error {
	ch, err := stdinRawCharPrompt(t, prompt)
	if err != nil {
		return err
	}

	switch ch {
	case 'q', 'Q', keyCtrlC, keyCtrlD, keyEsc:
		return promptui.ErrAbort
	}

	return nil
}

// stdinRawCharPrompt reads a single character from Stdin
func stdinRawCharPrompt(t Terminal, prompt string) (byte, error) {
	stdin := t.IOIn()

	// Display initial prompt
	t.Printf("%s", prompt)

	// Read a single byte from stdin. A closed stdin cannot answer,
	// which is the same as declining.
	var b [1]byte
	if n, err := readRaw(stdin, b[:]); errors.Is(err, io.EOF) {
		return 0, promptui.ErrAbort
	} else if err != nil {
		return 0, err
	} else if n == 0 {
		return 0, io.ErrNoProgress
	}

	// Add a newline, after success
	t.Printf("\n")

	// Return charaacter
	return b[0], nil
}

// readRaw reads from stdin in raw mode, so that a single key is read without
// waiting for Enter. Only a terminal can do this; a pipe or file is read
// as-is. The terminal is back to its previous mode before any output.
func readRaw(stdin io.Reader, b []byte) (int, error) {
	if fd, ok := terminalFd(stdin); ok {
		state, err := xterm.MakeRaw(fd)
		if err != nil {
			return 0, err
		}
		defer xterm.Restore(fd, state)
	}

	return stdin.Read(b)
}

// SpinIfTerminal shows a spinner on the error stream until the returned
// func is called. Like StartProgress, it does nothing on a non-terminal.
func SpinIfTerminal(t Terminal, suffix string) func() {
	ioErr := t.IOErr()
	if !isTerminal(ioErr) {
		return func() {}
	}
	spin := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(ioErr))
	spin.FinalMSG = "\r" + strings.Repeat(" ", 20) + "\r" // Erases previous string
	spin.Suffix = suffix
	spin.Start()
	return spin.Stop
}
