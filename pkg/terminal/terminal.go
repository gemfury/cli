package terminal

import (
	"github.com/gemfury/cli/pkg/browser"
	"github.com/manifoldco/promptui"

	"context"
	"fmt"
	"io"
	"os"
)

type Terminal interface {
	StartProgress(int64, string) Progress
	RunPrompt(*promptui.Prompt) (string, error)
	Confirm(label string) (bool, error)

	// Printf prints the result of a command, or what the user is asked
	Printf(string, ...any) (int, error)

	// Infof prints what is not the result of a command: a banner, the status
	// of a change, or that nothing was found. With --quiet it writes nothing,
	// and returns 0, which tells a caller that it was left out.
	Infof(string, ...any) (int, error)

	// EPrintf prints a warning, or the failure of an item, to the error stream
	EPrintf(string, ...any) (int, error)

	Spin(suffix string) func()
	OpenBrowser(context.Context, string) bool

	// IsInteractive reports whether there is a user to answer prompts: Stdin
	// is a terminal rather than a pipe, a file, or closed, and asking is
	// not turned off (see Unattended)
	IsInteractive() bool

	// IsOutTTY reports whether Stdout is a terminal rather than a pipe or
	// a file: whether a person reads what is printed, as it is printed
	IsOutTTY() bool

	IOIn() io.ReadCloser
	IOErr() io.Writer
	IOOut() io.Writer
}

func New() Terminal {
	return &term{
		ioErr: os.Stderr,
		ioOut: os.Stdout,
		ioIn:  os.Stdin,
	}
}

type term struct {
	ioErr io.WriteCloser
	ioOut io.WriteCloser
	ioIn  io.ReadCloser
}

func (t term) Printf(f string, a ...any) (int, error) {
	return fmt.Fprintf(t.ioOut, f, a...)
}

func (t term) Infof(f string, a ...any) (int, error) {
	return fmt.Fprintf(t.ioOut, f, a...)
}

func (t term) EPrintf(f string, a ...any) (int, error) {
	return fmt.Fprintf(t.ioErr, f, a...)
}

func (t term) IOErr() io.Writer {
	return t.ioErr
}

func (t term) IOOut() io.Writer {
	return t.ioOut
}

func (t term) IOIn() io.ReadCloser {
	return t.ioIn
}

func (t term) IsInteractive() bool {
	return isTerminal(t.ioIn)
}

func (t term) IsOutTTY() bool {
	return isTerminal(t.ioOut)
}

func (t term) RunPrompt(p *promptui.Prompt) (string, error) {
	p.Stdout = t.ioOut
	p.Stdin = t.ioIn
	return p.Run()
}

func (t term) OpenBrowser(ctx context.Context, url string) bool {
	return browser.Open(ctx, url)
}
