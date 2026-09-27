package terminal

import (
	"github.com/cheggaaa/pb/v3"
	xterm "golang.org/x/term"

	"io"
)

const (
	// Progress bar template to match legacy Gemfury CLI
	pbTemplate pb.ProgressBarTemplate = `{{string . "prefix"}}{{ bar . "[" "=" (cycle . "⠁" "⠂" "⠄" "⠂") " " "]" }} {{percent . }}`
)

// StartProgress renders a progress bar on the error stream. When that stream
// is not an interactive terminal (a pipe, a file, CI logs), no bar is shown,
// since the redraw sequences would only corrupt the output.
func (t term) StartProgress(size int64, prefix string) Progress {
	if !isTerminal(t.ioErr) {
		return noProgress{}
	}

	pBar := pbTemplate.New(0).SetTotal(size).SetWriter(t.ioErr)
	pBar = pBar.Set("prefix", prefix)
	pBar = pBar.Set(pb.CleanOnFinish, true)
	return &bar{pBar.Start()}
}

// isTerminal reports whether a stream (reader or writer) is backed by an
// interactive terminal. Anything without a file descriptor, such as the
// buffers used in tests, is not.
func isTerminal(stream any) bool {
	_, ok := terminalFd(stream)
	return ok
}

// terminalFd is the file descriptor of a stream that isTerminal
func terminalFd(stream any) (int, bool) {
	f, ok := stream.(interface{ Fd() uintptr })
	if !ok {
		return 0, false
	}
	fd := int(f.Fd())
	return fd, xterm.IsTerminal(fd)
}

type Progress interface {
	NewProxyReader(io.Reader) io.Reader
	Finish()
}

type bar struct {
	*pb.ProgressBar
}

func (b bar) NewProxyReader(r io.Reader) io.Reader {
	return b.ProgressBar.NewProxyReader(r)
}

func (b bar) Finish() {
	b.ProgressBar.Finish()
}

type noProgress struct{}

func (np noProgress) NewProxyReader(r io.Reader) io.Reader {
	return r
}

func (np noProgress) Finish() {
	// noop
}
