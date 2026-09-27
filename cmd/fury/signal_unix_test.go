//go:build unix

package main

import (
	"syscall"
	"testing"
	"time"
)

// A signal cancels the context, and is then there to set the exit status
func TestInterruptible(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			cc, caught := interruptible(t.Context())
			if err := syscall.Kill(syscall.Getpid(), sig); err != nil {
				t.Fatal(err)
			}

			select {
			case <-cc.Done():
			case <-time.After(5 * time.Second):
				t.Fatal("Expected the context to be cancelled")
			}

			if got := caught(); got != sig {
				t.Errorf("Expected %q to be caught, got %q", sig, got)
			}
		})
	}
}
