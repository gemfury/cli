package terminal

import (
	"github.com/cheggaaa/pb/v3"
	"io"
)

const (
	// Progress bar template to match legacy Gemfury CLI
	pbTemplate pb.ProgressBarTemplate = `{{string . "prefix"}}{{ bar . "[" "=" (cycle . "⠁" "⠂" "⠄" "⠂") " " "]" }} {{percent . }}`
)

var (
	// "Factory" for Gemfury-style progress bars
	pbFactory = pb.ProgressBarTemplate(pbTemplate)
)

func (t term) StartProgress(size int64, prefix string) Progress {
	pBar := pbFactory.Start64(size)
	pBar = pBar.Set("prefix", prefix)
	pBar = pBar.Set(pb.CleanOnFinish, true)
	return &bar{pBar}
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
