package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"context"
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
		Long: `Remove a package version, once confirmed.

If packages of different kinds share a name, prefix the kind, as
KIND:PACKAGE@VERSION. The kinds are:
  ` + packageKinds,
		Example: `  fury yank package@1.0.0
  fury yank package@1.0.0 other@2.1.0 --force
  fury yank package --version 1.0.0
  fury yank js:package@1.0.0`,
		Args: cobra.MatchAll(
			usageArgs(cobra.MinimumNArgs(1), "Please specify at least one package"),
			func(cmd *cobra.Command, args []string) error {
				if versionFlag != "" && len(args) > 1 {
					return usageErrorf("Use PACKAGE@VERSION for multiple yanks")
				}
				for _, arg := range args {
					if _, _, ok := yankTarget(arg, versionFlag); !ok {
						return usageErrorf("Invalid package/version specified: %s", arg)
					}
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
			lookups := newFailures(cc, len(args), "lookups", "Version")
			for _, arg := range args {
				if lookups.interrupted() {
					break
				}

				pkg, ver, _ := yankTarget(arg, versionFlag) // Valid by the Args check
				pkgVersions, err := filterVersions(cc, c, pkg, ver)
				if err != nil {
					lookups.add("looking up", pkg+"@"+ver, err)
					continue
				}
				versions = append(versions, pkgVersions...)
			}

			if err := lookups.err(); err != nil {
				return err
			} else if len(versions) == 0 {
				term.Infof("No matching versions found\n")
				return nil
			}

			if !forceFlag {
				termPrintVersions(term, versions)
				confirm := "Are you sure you want to delete these files? [y/N]"
				if ok, err := term.Confirm(confirm); !ok {
					return err
				}
			}

			removals := newFailures(cc, len(versions), "removals", "File")
			for _, v := range versions {
				if removals.interrupted() {
					break
				}

				if err := c.Yank(cc, v.Package.ID, v.ID); err != nil {
					removals.add("removing", v.Filename, err)
					continue
				}
				term.Infof("Removed %q\n", v.Filename)
			}

			return removals.err()
		},
	}

	// Flags and options
	yankCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Skip confirmation")
	yankCmd.Flags().StringVarP(&versionFlag, "version", "v", "", "Version, for a PACKAGE given without one")

	return yankCmd
}

// yankTarget is the package and version that an argument stands for:
// PACKAGE@VERSION, or a bare package name with the version of the flag.
// A package given as KIND:NAME must have a name.
func yankTarget(arg, versionFlag string) (pkg, ver string, ok bool) {
	pkg, ver, ok = arg, versionFlag, arg != ""
	if versionFlag == "" {
		pkg, ver, ok = splitPackageVersion(arg)
	}

	_, name, hasKind := strings.Cut(pkg, ":")
	return pkg, ver, ok && !(hasKind && name == "")
}

func filterVersions(cc context.Context, c *api.Client, pkg, ver string) ([]*api.Version, error) {
	versions := []*api.Version{}

	// Default search filters for listed versions
	filter := url.Values{"name": {pkg}, "version": {ver}}

	// Extract "kind:" from package name, if present
	if kind, name, ok := strings.Cut(pkg, ":"); ok && kind != "" {
		filter["name"] = []string{name}
		filter["kind"] = []string{kind}
	}

	// Paginate over version listings until no more pages
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
