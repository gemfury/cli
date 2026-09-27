package cli

import (
	"testing"
	"time"
)

// SetLoginPollTimeout shortens the wait for a browser login during a test
func SetLoginPollTimeout(t *testing.T, d time.Duration) {
	prev := loginPollTimeout
	t.Cleanup(func() { loginPollTimeout = prev })
	loginPollTimeout = d
}
