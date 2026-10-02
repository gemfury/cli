package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"bufio"
	"io"
	"strings"
)

// gitCredentialLimit is more than Git would send to describe a credential
const gitCredentialLimit = 64 * 1024

// NewCmdGitCredentials is the credential helper that Git runs for the
// repositories of Gemfury: see gitcredentials(7). Login sets it up where
// the session is saved in the system keychain, out of reach of Git.
func NewCmdGitCredentials() *cobra.Command {
	return &cobra.Command{
		Use:   "credentials OPERATION",
		Short: "Give the saved session to Git, as its credential helper",
		Long: `Give the saved session to Git, as its credential helper for git.fury.io.
Git runs this by itself, once "fury login" has saved a session in the
system keychain. The operation is "get": any other is ignored.`,
		Example:     `  fury git credentials get`,
		Hidden:      true,
		Args:        usageArgs(cobra.ExactArgs(1), "Please specify exactly one operation"),
		Annotations: skipAuth(),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()

			// Any operation but "get" is ignored. The session is saved and
			// removed by login and logout, not by the "store" and "erase"
			// of Git: a push that is refused must not log its user out.
			if args[0] != "get" {
				return nil
			}

			// Other hosts are not ours to answer for
			asked := readGitCredential(ctx.Terminal(cc).IOIn())
			if asked["protocol"] != "https" || asked["host"] != terminal.GitHost {
				return nil
			}

			// Without a session, Git is left to ask for the credentials
			user, token, err := ctx.Auther(cc).Auth()
			if err != nil || token == "" {
				return err
			}

			// A session of another user is of no use to the one asked for
			if name, ok := asked["username"]; ok && name != user {
				return nil
			}

			ctx.Terminal(cc).Printf("protocol=https\nhost=%s\nusername=%s\npassword=%s\n", terminal.GitHost, user, token)
			return nil
		},
	}
}

// readGitCredential reads the description of a credential, as Git writes it
// for a helper: lines of key=value, up to an empty one. Lines end at "\n"
// alone, so a carriage return stays in its value, which then matches nothing.
func readGitCredential(r io.Reader) map[string]string {
	attrs := map[string]string{}
	in := bufio.NewReader(io.LimitReader(r, gitCredentialLimit))

	for {
		line, err := in.ReadString('\n')
		line = strings.TrimSuffix(line, "\n")
		if key, value, ok := strings.Cut(line, "="); ok {
			attrs[key] = value
		}
		if line == "" || err != nil {
			return attrs
		}
	}
}
