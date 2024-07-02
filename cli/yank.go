package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/hashicorp/go-multierror"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// NewCmdYank generates the Cobra command for "yank"
func NewCmdYank() *cobra.Command {
	var versionFlag string
	var forceFlag bool

	yankCmd := &cobra.Command{
		Use:   "yank PACKAGE@VERSION",
		Short: "Remove a package version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("Please specify at least one package")
			}

			if versionFlag != "" && len(args) > 1 {
				return fmt.Errorf("Use PACKAGE@VERSION for multiple yanks")
			}

			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			versions := make([]*api.Version, 0, len(args))
			var multiErr *multierror.Error
			for _, pkg := range args {
				var ver string = ""

				if versionFlag != "" {
					ver = versionFlag
				} else if at := strings.LastIndex(pkg, "@"); at > 0 {
					pkg, ver = pkg[0:at], pkg[at+1:]
				}

				if pkg == "" || ver == "" {
					err := fmt.Errorf("Invalid package/version specified")
					multiErr = multierror.Append(multiErr, err)
					continue
				}

				pkgVersions, err := filterVersions(cc, c, pkg, ver)
				versions = append(versions, pkgVersions...)
				multiErr = multierror.Append(multiErr, err)
			}

			if err := multiErr.Unwrap(); err != nil {
				return err
			} else if len(versions) == 0 {
				term.Printf("No matching versions found\n")
				return nil
			}

			if !forceFlag {
				termPrintVersions(term, versions)
				prompt := promptui.Prompt{
					Label:     "Are you sure you want to delete these files? [y/N]",
					IsConfirm: true,
				}
				_, err := term.RunPrompt(&prompt)
				if errors.Is(err, promptui.ErrAbort) {
					return nil
				} else if err != nil {
					return err
				}
			}

			for _, v := range versions {
				err = c.Yank(cc, v.Package.ID, v.ID)
				if err != nil {
					multiErr = multierror.Append(multiErr, err)
					continue
				}
				term.Printf("Removed %q\n", v.Filename)
			}

			return multiErr.Unwrap()
		},
	}

	// Flags and options
	yankCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Skip confirmation")
	yankCmd.Flags().StringVarP(&versionFlag, "version", "v", "", "Version")

	return yankCmd
}

func filterVersions(cc context.Context, c *api.Client, pkg, ver string) ([]*api.Version, error) {
	versions := []*api.Version{}

	// Default search filters for listed versions
	filter := url.Values(map[string][]string{"name": {pkg}, "version": {ver}})

	// Extract "kind:" from package name, if present
	if at := strings.Index(pkg, ":"); at > 0 {
		filter["name"] = []string{pkg[at+1:]}
		filter["kind"] = []string{pkg[0:at]}
	}

	// Paginate over package listings until no more pages
	err := iterateAllPages(cc, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
		resp, err := c.Versions(cc, filter, pageReq)
		if err != nil {
			return nil, err
		}
		versions = append(versions, resp.Versions...)
		return resp.Pagination, nil
	})

	return versions, err
}
