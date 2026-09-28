package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
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
			fails := newFailures(cc, len(args), "uploads", "File")
			for _, path := range args {
				if fails.interrupted() {
					break
				}

				name := filepath.Base(path)
				prefix := fmt.Sprintf("Uploading %s ", name)

				err := func() error {
					file, err := os.Open(path)
					if err != nil {
						return err
					}
					defer file.Close()

					// A directory opens fine, but there is nothing to upload
					stat, err := file.Stat()
					if err != nil {
						return err
					} else if stat.IsDir() {
						return &fs.PathError{Op: "read", Path: path, Err: syscall.EISDIR}
					}

					// Prepare progress bar
					var reader io.Reader = file
					if noProgress {
						term.Printf("%s", prefix)
						prefix = ""
					} else {
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

				fails.record(name, err) // Reported by the status line below
				if errors.Is(err, context.Canceled) {
					term.Printf("%s- cancelled\n", prefix)
				} else if errors.Is(err, fs.ErrNotExist) {
					term.Printf("%s- file not found\n", prefix)
				} else if errors.Is(err, syscall.EISDIR) {
					term.Printf("%s- is a directory\n", prefix)
				} else if errors.Is(err, api.ErrUnauthorized) {
					term.Printf("%s- unauthorized\n", prefix)
				} else if errors.Is(err, api.ErrForbidden) {
					term.Printf("%s- no permission\n", prefix)
				} else if errors.Is(err, api.ErrAlreadyExists) {
					term.Printf("%s- this version already exists\n", prefix)
				} else if ue := (api.UserError{}); errors.As(err, &ue) {
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
