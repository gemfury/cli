package cli

import (
	"testing"
	"time"
)

// SetLoginPollTimeout shortens the wait for a browser login during a test
func SetLoginPollTimeout(t *testing.T, d time.Duration) {
	setDuring(t, &loginPollTimeout, d)
}

// SetLoginPollInterval shortens the wait between polls during a test
func SetLoginPollInterval(t *testing.T, d time.Duration) {
	setDuring(t, &loginPollInterval, d)
}

// setDuring sets a variable until the end of a test, which restores it
func setDuring[T any](t *testing.T, v *T, to T) {
	prev := *v
	t.Cleanup(func() { *v = prev })
	*v = to
}

// AsUsageError makes a usage error of err, which remains its cause
func AsUsageError(err error) error {
	return asUsageError(err)
}
