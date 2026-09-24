package terminal

import (
	"github.com/briandowns/spinner"
	"github.com/chzyer/readline"
	"github.com/manifoldco/promptui"

	"errors"
	"io"
	"strings"
	"time"
)

// PromptConfirm asks a "y/N" question from Stdin
func PromptConfirm(t Terminal, label string) (bool, error) {
	_, err := t.RunPrompt(&promptui.Prompt{Label: label, IsConfirm: true})
	if errors.Is(err, promptui.ErrAbort) {
		return false, nil
	}
	return err == nil, err
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

	// Enter raw mode to read a single key without waiting for Enter.
	// Only a terminal can do this; a pipe or file is read as-is.
	if isTerminal(stdin) {
		rm := new(readline.RawMode)
		if err := rm.Enter(); err != nil {
			return 0, err
		}
		defer rm.Exit()
	}

	// Display initial prompt
	t.Printf("%s", prompt)

	// Read a single byte from stdin. A closed stdin cannot answer,
	// which is the same as declining.
	var b [1]byte
	if n, err := stdin.Read(b[:]); errors.Is(err, io.EOF) {
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
