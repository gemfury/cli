package main

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"

	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"syscall"
	"testing"
)

// A legacy colon-joined command is split into subcommands, with its
// flags as they are, unless a flag comes first
func TestConvertLegacyArgs(t *testing.T) {
	for _, tc := range []struct {
		args, want []string
	}{
		{[]string{"fury", "git:list", "--json"}, []string{"git", "list", "--json"}},
		{[]string{"fury", "git:config:get", "repo", "KEY", "--json"}, []string{"git", "config", "get", "repo", "KEY", "--json"}},
		{[]string{"fury", "--json", "git", "list"}, nil},
	} {
		if got := convertLegacyArgs(tc.args); !slices.Equal(got, tc.want) {
			t.Errorf("Legacy args %q = %q, want %q", tc.args, got, tc.want)
		}
	}
}

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
		"failure, of its status":  {api.ErrNotFound, nil, cli.ExitNotFound},
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
