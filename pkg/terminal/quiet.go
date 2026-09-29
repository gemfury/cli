package terminal

// quiet is a Terminal that shows less
type quiet struct {
	Terminal
	hideInfo     bool
	hideProgress bool
}

// Quiet wraps a Terminal to leave out what Infof prints, with --quiet,
// and progress bars and spinners, with --quiet or --no-progress
func Quiet(t Terminal, isQuiet, isNoProgress bool) Terminal {
	return quiet{Terminal: t, hideInfo: isQuiet, hideProgress: isQuiet || isNoProgress}
}

func (q quiet) Infof(f string, a ...any) (int, error) {
	if q.hideInfo {
		return 0, nil
	}
	return q.Terminal.Infof(f, a...)
}

func (q quiet) StartProgress(size int64, prefix string) Progress {
	if q.hideProgress {
		return noProgress{}
	}
	return q.Terminal.StartProgress(size, prefix)
}

func (q quiet) Spin(suffix string) func() {
	if q.hideProgress {
		return func() {}
	}
	return q.Terminal.Spin(suffix)
}
