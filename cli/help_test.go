package cli_test

import (
	"github.com/gemfury/cli/cli"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"context"
	"strings"
	"testing"
)

// exampleCommand finds the command that args run, and validates its flags and
// arguments. Each call builds its own root, as parsed flags keep their values.
func exampleCommand(cc context.Context, args []string) (*cobra.Command, error) {
	found, rest, err := cli.NewRootCommand(cc).Find(args)
	if err != nil {
		return nil, err
	} else if err := found.ParseFlags(rest); err != nil {
		return nil, err
	}
	return found, found.ValidateArgs(found.Flags().Args())
}

// exampleArgs are the arguments of an example of the command called name,
// past that name and any VAR=value before it
func exampleArgs(line, name string) (args []string, ok bool) {
	args = strings.Fields(line)
	for len(args) > 0 && strings.Contains(args[0], "=") {
		args = args[1:]
	}
	if len(args) == 0 || args[0] != name {
		return nil, false
	}
	return args[1:], true
}

// Help never requires authentication, and never contacts the API
func TestHelpWithoutAuth(t *testing.T) {
	for _, command := range []string{"help", "help push", "git --help", "git", "beta"} {
		t.Run(command, func(t *testing.T) {
			if out := helpOutput(t, strings.Fields(command)...); !strings.Contains(out, "Usage:") {
				t.Errorf("Expected help on stdout, got %q", out)
			}
		})
	}
}

// Every command with an action has examples, as help is all that an agent
// learns from. Each is a valid command line for that command. The examples
// of the root are for its subcommands; other groups have none.
func TestHelpExamples(t *testing.T) {
	// Commands are parsed and never run, so they need no API
	cc := cli.TestContext(t.Context(), terminal.NewForTest(), terminal.TestAuther("", "", nil))
	root := cli.NewRootCommand(cc)

	for _, cmd := range allCommands(root) {
		path := cmd.CommandPath()
		t.Run(path, func(t *testing.T) {
			if cli.IsGroup(cmd) {
				if cmd.HasExample() {
					t.Error("Expected no examples for a group")
				}
				return
			} else if !cmd.HasExample() {
				t.Fatal("Expected examples in help")
			}

			for _, line := range strings.Split(cmd.Example, "\n") {
				if line != "  "+strings.TrimSpace(line) {
					t.Errorf("Expected an indent of two spaces, got %q", line)
				}

				args, ok := exampleArgs(line, root.Name())
				if !ok {
					t.Fatalf("Expected a command line of %q, got %q", root.Name(), line)
				}

				found, err := exampleCommand(cc, args)
				if err != nil {
					t.Fatalf("Example %q: %s", line, err)
				} else if cmd != root && found.CommandPath() != path {
					t.Errorf("Expected an example of %q, got %q", path, line)
				}
			}
		})
	}
}

// Help says what cannot be guessed: the values of a flag, what
// the account is, and how the kind of a package is given
func TestHelpValues(t *testing.T) {
	for command, exps := range map[string][]string{
		"fury":             {"--api-token, or else by FURY_TOKEN"},
		"fury accounts":    {"can be given to --account"},
		"fury packages":    {"Account to act on, if not your own"},
		"fury sharing add": {"pull, push, or owner"},
		"fury beta backup": {cli.PackageKinds},
		"fury yank":        {cli.PackageKinds, "KIND:PACKAGE@VERSION"},
	} {
		t.Run(command, func(t *testing.T) {
			out := helpOutput(t, append(strings.Fields(command)[1:], "--help")...)
			for _, exp := range exps {
				if !strings.Contains(out, exp) {
					t.Errorf("Expected %q in help, got %q", exp, out)
				}
			}
		})
	}
}
