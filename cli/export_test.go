package cli

import (
	"github.com/spf13/cobra"

	"context"
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

// ClientTimeouts exposes the timeouts of the API client built for a command
func ClientTimeouts(t *testing.T, cc context.Context) (request, long time.Duration) {
	t.Helper()
	c, err := newAPIClientWithToken(cc, "token")
	if err != nil {
		t.Fatal(err)
	}
	return c.Timeout, c.LongTimeout
}

// setDuring sets a variable until the end of a test, which restores it
func setDuring[T any](t *testing.T, v *T, to T) {
	prev := *v
	t.Cleanup(func() { *v = prev })
	*v = to
}

// PackageKinds exposes the package kinds listed in help
const PackageKinds = packageKinds

// AgoString exposes how long ago a time is said to be at a terminal
var AgoString = agoString

// IsGroup reports whether cmd only groups subcommands (see groupCommand)
func IsGroup(cmd *cobra.Command) bool {
	return cmd.Annotations[groupKey] == "true"
}

// AsUsageError makes a usage error of err, which remains its cause
func AsUsageError(err error) error {
	return asUsageError(err)
}
