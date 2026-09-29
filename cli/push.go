package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// NewCmdPush generates the Cobra command for "push"
func NewCmdPush() *cobra.Command {
	var isPublic bool

	pushCmd := &cobra.Command{
		Use:   "push FILE...",
		Short: "Upload a new version of a package",
		Example: `  fury push package-1.0.0.tgz
  fury push --public package-1.0.0.gem package-1.1.0.gem
  fury push --quiet package.deb --account my-org`,
		Args: usageArgs(cobra.MinimumNArgs(1), "Please specify at least one package file"),
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
					bar := term.StartProgress(stat.Size(), prefix)
					defer bar.Finish()

					return c.PushPkg(cc, name, isPublic, bar.NewProxyReader(file))
				}()

				if err == nil {
					term.Infof("%s- done\n", prefix)
					continue
				}

				// Reported by the status line, or else on stderr
				// when --quiet left that out, and nothing was written
				if n, _ := term.Infof("%s- %s\n", prefix, pushStatus(err)); n > 0 {
					fails.record(name, err)
				} else {
					fails.add("uploading", name, err)
				}
			}

			return fails.err()
		},
	}

	// Flags and options
	pushCmd.Flags().BoolVar(&isPublic, "public", false, "Create as public package")

	return pushCmd
}

// pushStatus ends the status line of a file that failed to upload
func pushStatus(err error) string {
	ue := api.UserError{}
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, fs.ErrNotExist):
		return "file not found"
	case errors.Is(err, syscall.EISDIR):
		return "is a directory"
	case errors.Is(err, api.ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, api.ErrForbidden):
		return "no permission"
	case errors.Is(err, api.ErrAlreadyExists):
		return "this version already exists"
	case errors.As(err, &ue):
		return ue.ShortError()
	}
	return fmt.Sprintf("error %q", err.Error())
}
