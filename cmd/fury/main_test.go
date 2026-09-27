package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestExitStatus(t *testing.T) {
	cancelled := fmt.Errorf("Request failed: %w", context.Canceled)

	for name, tc := range map[string]struct {
		err error
		sig os.Signal
		exp int
	}{
		"success":                 {nil, nil, 0},
		"success, late signal":    {nil, syscall.SIGTERM, 0},
		"failure":                 {errors.New("Failed"), nil, 1},
		"failure, late signal":    {errors.New("Failed"), syscall.SIGINT, 1},
		"interrupted by SIGINT":   {cancelled, syscall.SIGINT, 130},
		"interrupted by SIGTERM":  {cancelled, syscall.SIGTERM, 143},
		"interrupted at a prompt": {context.Canceled, nil, 130},
	} {
		t.Run(name, func(t *testing.T) {
			if got := exitStatus(tc.err, tc.sig); got != tc.exp {
				t.Errorf("Expected exit status %d, got %d", tc.exp, got)
			}
		})
	}
}
