package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"context"
	"fmt"
	"net/url"
	"strings"
)

// NewCmdYank generates the Cobra command for "yank"
func NewCmdYank() *cobra.Command {
	var versionFlag string
	var forceFlag bool

	yankCmd := &cobra.Command{
		Use:   "yank PACKAGE@VERSION...",
		Short: "Remove a package version",
		Args: cobra.MatchAll(
			usageArgs(cobra.MinimumNArgs(1), "Please specify at least one package"),
			func(cmd *cobra.Command, args []string) error {
				if versionFlag != "" && len(args) > 1 {
					return usageErrorf("Use PACKAGE@VERSION for multiple yanks")
				}
				return nil
			},
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			// Resolve every argument before removing anything
			versions := make([]*api.Version, 0, len(args))
			lookups := newFailures(term, len(args), "lookups")
			for _, arg := range args {
				pkgVersions, err := lookupVersions(cc, c, arg, versionFlag)
				if err != nil {
					lookups.add("looking up", arg, err)
					continue
				}
				versions = append(versions, pkgVersions...)
			}

			if err := lookups.err(); err != nil {
				return err
			} else if len(versions) == 0 {
				term.Printf("No matching versions found\n")
				return nil
			}

			if !forceFlag {
				termPrintVersions(term, versions)
				confirm := "Are you sure you want to delete these files? [y/N]"
				if ok, err := terminal.PromptConfirm(term, confirm); !ok {
					return err
				}
			}

			removals := newFailures(term, len(versions), "removals")
			for _, v := range versions {
				if err := c.Yank(cc, v.Package.ID, v.ID); err != nil {
					removals.add("removing", v.Filename, err)
					continue
				}
				term.Printf("Removed %q\n", v.Filename)
			}

			return removals.err()
		},
	}

	// Flags and options
	yankCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Skip confirmation")
	yankCmd.Flags().StringVarP(&versionFlag, "version", "v", "", "Version")

	return yankCmd
}

// lookupVersions resolves one yank argument to its versions. The argument
// is PACKAGE@VERSION, or a bare package name when the version comes from
// the --version flag.
func lookupVersions(cc context.Context, c *api.Client, arg, versionFlag string) ([]*api.Version, error) {
	pkg, ver := arg, versionFlag
	if at := strings.LastIndex(arg, "@"); versionFlag == "" && at > 0 {
		pkg, ver = arg[0:at], arg[at+1:]
	}

	if pkg == "" || ver == "" {
		return nil, fmt.Errorf("Invalid package/version specified")
	}

	return filterVersions(cc, c, pkg, ver)
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
