package cli

import (
	"github.com/cheggaaa/pb/v3"
	"github.com/gemfury/cli/api"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// Progress bar template to match legacy Gemfury CLI
	pbTemplate pb.ProgressBarTemplate = `{{string . "prefix"}}{{ bar . "[" "=" (cycle . "⠁" "⠂" "⠄" "⠂") " " "]" }} {{percent . }}`
)

// NewCmdPush generates the Cobra command for "push"
func NewCmdPush() *cobra.Command {
	var noProgress bool

	pushCmd := &cobra.Command{
		Use:   "push PACKAGE",
		Short: "Upload a new version of a package",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Please specify at least one package")
			}

			cc := cmd.Context()
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			// Initialize progress bar template
			pbFactory := pb.ProgressBarTemplate(pbTemplate)

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
						fmt.Print(prefix)
						prefix = ""
					} else {
						stat, _ := file.Stat()
						bar := pbFactory.Start64(stat.Size())
						bar = bar.Set("prefix", prefix)
						bar = bar.Set(pb.CleanOnFinish, true)
						reader = bar.NewProxyReader(file)
						defer bar.Finish()
					}

					err = c.PushPkg(cc, name, reader)
					return err
				}()

				if err != nil {
					multiErr = multierror.Append(multiErr, err)
				}

				if err == nil {
					fmt.Printf("%s- done\n", prefix)
				} else if os.IsNotExist(err) {
					fmt.Printf("%s- file not found\n", prefix)
				} else if errors.Is(err, api.ErrClientAuth) {
					fmt.Printf("%s- unauthorized\n", prefix)
				} else if err.Error() == "Conflict" {
					fmt.Printf("%s- this version already exists\n", prefix)
				} else {
					fmt.Printf("%s- error %q\n", prefix, err.Error())
				}
			}

			if multiErr != nil {
				cmd.SilenceUsage = true
				cmd.SilenceErrors = true
				multiErr.ErrorFormat = func([]error) string {
					return "There was a problem uploading at least 1 package"
				}
			}

			return multiErr.ErrorOrNil()
		},
	}

	// Flags and options
	pushCmd.Flags().BoolVar(&noProgress, "quiet", false, "Do not show progress bar")

	return pushCmd
}
