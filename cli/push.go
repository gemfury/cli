package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/hashicorp/go-multierror"
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
		Use:   "push PACKAGE",
		Short: "Upload a new version of a package",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Please specify at least one package")
			}

			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			// Upload each file and collect errors
			var multiErr *multierror.Error
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
						term.Printf(prefix)
						prefix = ""
					} else {
						stat, _ := file.Stat()
						bar := term.StartProgress(stat.Size(), prefix)
						reader = bar.NewProxyReader(file)
						defer bar.Finish()
					}

					err = c.PushPkg(cc, name, isPublic, reader)
					return err
				}()

				if err != nil {
					multiErr = multierror.Append(multiErr, err)
				}

				if err == nil {
					term.Printf("%s- done\n", prefix)
				} else if os.IsNotExist(err) {
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

			if multiErr != nil {
				cmd.SilenceUsage = true
				cmd.SilenceErrors = true
				multiErr.ErrorFormat = func([]error) string {
					return "There was a problem uploading at least 1 package"
				}
			}

			return multiErr.Unwrap()
		},
	}

	// Flags and options
	pushCmd.Flags().BoolVar(&noProgress, "quiet", false, "Do not show progress bar")
	pushCmd.Flags().BoolVar(&isPublic, "public", false, "Create as public package")

	return pushCmd
}
