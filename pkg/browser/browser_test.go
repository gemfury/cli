package browser

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestAppearsSuccessful(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("No sleep command")
	}

	start := func(t *testing.T, arg string) *exec.Cmd {
		t.Helper()
		cmd := exec.Command(sleep, arg)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cmd.Process.Kill() })
		return cmd
	}

	t.Run("exited cleanly", func(t *testing.T) {
		if !appearsSuccessful(t.Context(), start(t, "0"), time.Minute) {
			t.Error("Expected success for a clean exit")
		}
	})

	t.Run("still running", func(t *testing.T) {
		if !appearsSuccessful(t.Context(), start(t, "60"), 50*time.Millisecond) {
			t.Error("Expected success for a command outliving the timeout")
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		cc, cancel := context.WithCancel(t.Context())
		cancel()

		began := time.Now()
		if appearsSuccessful(cc, start(t, "60"), time.Minute) {
			t.Error("Expected no success once cancelled")
		} else if d := time.Since(began); d > 5*time.Second {
			t.Errorf("Expected a prompt return, took %s", d)
		}
	})
}

func TestOpenCancelled(t *testing.T) {
	cc, cancel := context.WithCancel(t.Context())
	cancel()

	// Nothing is launched for a context that is already done
	if Open(cc, "https://gemfury.com") {
		t.Error("Expected no browser for a cancelled context")
	}
}
