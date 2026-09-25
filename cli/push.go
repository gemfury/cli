package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// NewCmdPush generates the Cobra command for "push"
func NewCmdPush() *cobra.Command {
	var noProgress bool
	var isPublic bool

	pushCmd := &cobra.Command{
		Use:   "push FILE...",
		Short: "Upload a new version of a package",
		Args:  usageArgs(cobra.MinimumNArgs(1), "Please specify at least one package file"),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			// Upload each file and collect errors
			fails := newFailures(term, len(args), "uploads")
			for _, path := range args {
				name := filepath.Base(path)
				prefix := fmt.Sprintf("Uploading %s ", name)

				err := func() error {
					file, err := os.Open(path)
					if err != nil {
						return err
					}
					defer file.Close()

					// Prepare progress bar
					var reader io.Reader = file
					if noProgress {
						term.Printf("%s", prefix)
						prefix = ""
					} else {
						stat, _ := file.Stat()
						bar := term.StartProgress(stat.Size(), prefix)
						reader = bar.NewProxyReader(file)
						defer bar.Finish()
					}

					return c.PushPkg(cc, name, isPublic, reader)
				}()

				if err == nil {
					term.Printf("%s- done\n", prefix)
					continue
				}

				fails.record(err) // Reported by the status line below
				if os.IsNotExist(err) {
					term.Printf("%s- file not found\n", prefix)
				} else if errors.Is(err, api.ErrUnauthorized) {
					term.Printf("%s- unauthorized\n", prefix)
				} else if errors.Is(err, api.ErrForbidden) {
					term.Printf("%s- no permission\n", prefix)
				} else if ue, ok := err.(api.UserError); ok {
					term.Printf("%s- %s\n", prefix, ue.ShortError())
				} else {
					term.Printf("%s- error %q\n", prefix, err.Error())
				}
			}

			// Per-file status is already on stdout; Execute reports the error
			return fails.err()
		},
	}

	// Flags and options
	pushCmd.Flags().BoolVar(&noProgress, "quiet", false, "Do not show progress bar")
	pushCmd.Flags().BoolVar(&isPublic, "public", false, "Create as public package")

	return pushCmd
}
