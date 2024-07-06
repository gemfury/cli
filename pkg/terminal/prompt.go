package terminal

import (
	"github.com/briandowns/spinner"
	"github.com/chzyer/readline"
	"github.com/manifoldco/promptui"

	"errors"
	"io"
	"os"
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

// PromptAnyKeyOrQuit reads either "q" or any key from Stdin
func PromptAnyKeyOrQuit(t Terminal, prompt string) error {
	if ch, err := stdinRawCharPrompt(t, prompt); err != nil {
		return err
	} else if ch == 113 || ch == 81 { // "Q" or "q"
		return promptui.ErrAbort
	}
	return nil
}

// stdinRawCharPrompt reads a single character from Stdin
func stdinRawCharPrompt(t Terminal, prompt string) (byte, error) {
	stdin := t.IOIn()

	// Enter raw mode, for actual STDIN
	if stdin == os.Stdin {
		rm := new(readline.RawMode)
		if err := rm.Enter(); err != nil {
			return 0, err
		}
		defer rm.Exit()
	}

	// Display initial prompt
	t.Printf(prompt)

	// Read a single byte from stdin
	var b [1]byte
	if n, err := stdin.Read(b[:]); err != nil {
		return 0, err
	} else if n == 0 {
		return 0, io.ErrNoProgress
	}

	// Add a newline, after success
	t.Printf("\n")

	// Return charaacter
	return b[0], nil
}

// IsTerminal true if IOOut is terminal Stdout
func SpinIfTerminal(t Terminal, suffix string) func() {
	ioErr := t.IOErr() // can be real os.Stderr or placeholder for testing
	if osErr := os.Stderr; ioErr != osErr || !readline.IsTerminal(int(osErr.Fd())) {
		return func() {} // IOErr is not a TTY terminal
	}
	spin := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(ioErr))
	spin.FinalMSG = "\r" + strings.Repeat(" ", 20) + "\r" // Erases previous string
	spin.Suffix = suffix
	spin.Start()
	return func() {
		spin.Stop()
		t.Printf("\r")
	}
}
