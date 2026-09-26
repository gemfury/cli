package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"context"
	"fmt"
	"text/tabwriter"
)

// NewCmdPackages creates the "packages" command
func NewCmdPackages() *cobra.Command {
	return &cobra.Command{
		Use:     "packages",
		Aliases: []string{"list"},
		Short:   "List packages in this account",
		Args:    noArgs,
		RunE:    listPackages,
	}
}

// NewCmdVersions creates the "versions" command
func NewCmdVersions() *cobra.Command {
	return &cobra.Command{
		Use:   "versions PACKAGE",
		Short: "List versions for a package",
		Args:  usageArgs(cobra.ExactArgs(1), "Please specify exactly one package"),
		RunE:  listVersions,
	}
}

func listPackages(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	packages := []*api.Package{}

	// Paginate over package listings until no more pages
	err = iterateAllPages(cc, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
		resp, err := c.Packages(cc, pageReq)
		if err != nil {
			return nil, err
		}

		packages = append(packages, resp.Packages...)
		return resp.Pagination, nil
	})

	if noResults(term, len(packages), err, "No packages found in this account") {
		return err
	}

	// Print results
	term.Printf("\n*** GEMFURY PACKAGES ***\n\n")
	w := tabwriter.NewWriter(term.IOOut(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "name\tkind\tversion\tprivacy\n")

	for _, p := range packages {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Kind, p.DisplayVersion(), p.Privacy())
	}

	w.Flush()
	return err
}

func listVersions(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	versions := []*api.Version{}

	// Paginate over package listings until no more pages
	err = iterateAllPages(cc, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
		resp, err := c.PackageVersions(cc, args[0], pageReq)
		if err != nil {
			return nil, err
		}

		versions = append(versions, resp.Versions...)
		return resp.Pagination, nil
	})

	if noResults(term, len(versions), err, fmt.Sprintf("No versions found for package %q", args[0])) {
		return err
	}

	// Print results
	term.Printf("\n*** %s versions ***\n\n", args[0])
	termPrintVersions(term, versions)
	return err
}

func termPrintVersions(term terminal.Terminal, versions []*api.Version) {
	w := tabwriter.NewWriter(term.IOOut(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "version\tuploaded_by\tuploaded_at\tkind\tfilename\n")

	for _, v := range versions {
		uploadedAt := timeStringWithAgo(v.CreatedAt)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", v.Version, v.DisplayCreatedBy(), uploadedAt, v.Kind(), v.Filename)
	}

	w.Flush()
}

func iterateAllPages(cc context.Context, fn func(req *api.PaginationRequest) (*api.PaginationResponse, error)) error {
	return iterateAll(cc, true, fn)
}

func iterateAll(cc context.Context, showSpinner bool, fn func(req *api.PaginationRequest) (*api.PaginationResponse, error)) error {
	term := ctx.Terminal(cc)
	pageReq := api.PaginationRequest{
		Limit: 100,
	}

	// Spinner is shown only on a TTY, and only from the second page on
	var stopSpinner func()
	defer func() {
		if stopSpinner != nil {
			stopSpinner()
		}
	}()

	for {
		pageResp, err := fn(&pageReq)
		if err != nil {
			return err
		}

		pageReq.Page = ""
		if pageResp != nil {
			pageReq.Page = pageResp.NextPageCursor()
		}

		if pageReq.Page == "" || cc.Err() != nil {
			break
		}

		if stopSpinner == nil && showSpinner {
			stopSpinner = terminal.SpinIfTerminal(term, " Fetching ...")
		}
	}

	return nil
}
