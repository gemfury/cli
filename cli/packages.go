package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/spf13/cobra"

	"context"
	"fmt"
	"os"
	"text/tabwriter"
)

// Root for sharing/collaboration subcommands
func NewCmdPackages() *cobra.Command {
	return &cobra.Command{
		Use:     "packages",
		Aliases: []string{"list"},
		Short:   "Package listing",
		RunE:    listPackages,
	}
}

func NewCmdVersions() *cobra.Command {
	return &cobra.Command{
		Use:   "versions PACKAGE",
		Short: "Package versions",
		RunE:  listVersions,
	}
}

func listPackages(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
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

	// Handle no packages
	if len(packages) == 0 {
		fmt.Println("No packages found in this account")
		return nil
	}

	// Print results
	fmt.Printf("\n*** GEMFURY PACKAGES ***\n\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "name\tkind\tversion\tprivacy\n")

	for _, p := range packages {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Kind, p.DisplayVersion(), p.Privacy())
	}

	w.Flush()
	return nil
}

func listVersions(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Please specify a package")
	}

	cc := cmd.Context()
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	versions := []*api.Version{}

	// Paginate over package listings until no more pages
	err = iterateAllPages(cc, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
		resp, err := c.Versions(cc, args[0], pageReq)
		if err != nil {
			return nil, err
		}

		versions = append(versions, resp.Versions...)
		return resp.Pagination, nil
	})

	// Print results
	fmt.Printf("\n*** %s versions ***\n\n", args[0])
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "version\tuploaded_by\tuploaded_at\n")

	for _, v := range versions {
		uploadedAt := v.CreatedAt.Local().Format("2006-01-02 15:04")
		fmt.Fprintf(w, "%s\t%s\t%s\n", v.Version, v.DisplayCreatedBy(), uploadedAt)
	}

	w.Flush()
	return nil
}

func iterateAllPages(cc context.Context, fn func(req *api.PaginationRequest) (*api.PaginationResponse, error)) error {
	pageReq := api.PaginationRequest{
		Limit: 50,
	}

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
	}

	return nil
}
