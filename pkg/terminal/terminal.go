package terminal

import (
	"github.com/gemfury/cli/pkg/browser"
	"github.com/manifoldco/promptui"

	"fmt"
	"io"
	"os"
)

type Terminal interface {
	StartProgress(int64, string) Progress
	RunPrompt(*promptui.Prompt) (string, error)
	Printf(string, ...interface{}) (int, error)
	Println(a ...interface{}) (n int, err error)
	OpenBrowser(string) bool
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

func (t term) Printf(f string, a ...interface{}) (int, error) {
	return fmt.Fprintf(t.ioOut, f, a...)
}

func (t term) Println(a ...interface{}) (int, error) {
	return fmt.Fprintln(t.ioOut, a...)
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

func (t term) RunPrompt(p *promptui.Prompt) (string, error) {
	p.Stdout = t.ioOut
	p.Stdin = t.ioIn
	return p.Run()
}

func (t term) OpenBrowser(url string) bool {
	return browser.Open(url)
}
