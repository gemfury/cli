package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"context"
	"fmt"
	"text/tabwriter"
)

// NewCmdPackages creates the "packages" command
func NewCmdPackages() *cobra.Command {
	return jsonCommand(&cobra.Command{
		Use:     "packages",
		Aliases: []string{"list"},
		Short:   "List packages in this account",
		Example: `  fury packages
  fury packages --account my-org`,
		Args: noArgs,
		RunE: listPackages,
	})
}

// NewCmdVersions creates the "versions" command
func NewCmdVersions() *cobra.Command {
	return jsonCommand(&cobra.Command{
		Use:     "versions PACKAGE",
		Short:   "List versions for a package",
		Example: `  fury versions package`,
		Args:    usageArgs(cobra.ExactArgs(1), "Please specify exactly one package"),
		RunE:    listVersions,
	})
}

func listPackages(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	packages, err := fetchAll[*api.Package](cc, c.Packages)
	return printListing(cmd, packages, err, "No packages found in this account", func(term terminal.Terminal) {
		term.Infof("\n*** GEMFURY PACKAGES ***\n\n")
		w := tabwriter.NewWriter(term.IOOut(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "name\tkind\tversion\tprivacy\n")

		for _, p := range packages {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Kind, p.DisplayVersion(), p.Privacy())
		}

		w.Flush()
	})
}

func listVersions(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	versions, err := fetchAll[*api.Version](cc, func(cc context.Context, pageReq *api.PaginationRequest) (*api.VersionsResponse, error) {
		return c.PackageVersions(cc, args[0], pageReq)
	})
	err = about("Package", args[0], err)

	empty := fmt.Sprintf("No versions found for package %q", args[0])
	return printListing(cmd, versions, err, empty, func(term terminal.Terminal) {
		term.Infof("\n*** %s versions ***\n\n", args[0])
		termPrintVersions(term, versions)
	})
}

func termPrintVersions(term terminal.Terminal, versions []*api.Version) {
	w := tabwriter.NewWriter(term.IOOut(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "version\tcreated_by\tcreated_at\tkind\tfilename\n")

	tty := term.IsOutTTY()
	for _, v := range versions {
		createdAt := timeString(v.CreatedAt, tty)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", v.Version, v.DisplayCreatedBy(), createdAt, v.DisplayKind(), v.Filename)
	}

	w.Flush()
}
