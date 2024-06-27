package cli

import (
	"github.com/briandowns/spinner"
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// NewCmdPackages creates the "packages" command
func NewCmdPackages() *cobra.Command {
	return &cobra.Command{
		Use:     "packages",
		Aliases: []string{"list"},
		Short:   "List packages in this account",
		RunE:    listPackages,
	}
}

// NewCmdVersions creates the "versions" command
func NewCmdVersions() *cobra.Command {
	return &cobra.Command{
		Use:   "versions PACKAGE",
		Short: "List versions for a package",
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

	// Handle no packages
	if len(packages) == 0 {
		term.Println("No packages found in this account")
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
	if len(args) != 1 {
		return fmt.Errorf("Please specify a package")
	}

	cc := cmd.Context()
	term := ctx.Terminal(cc)
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
	term.Printf("\n*** %s versions ***\n\n", args[0])
	w := tabwriter.NewWriter(term.IOOut(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "version\tuploaded_by\tuploaded_at\tfilename\n")

	for _, v := range versions {
		uploadedAt := timeStringWithAgo(v.CreatedAt)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", v.Version, v.DisplayCreatedBy(), uploadedAt, v.Filename)
	}

	w.Flush()
	return err
}

func iterateAllPages(cc context.Context, fn func(req *api.PaginationRequest) (*api.PaginationResponse, error)) error {
	return iterateAll(cc, true, fn)
}

func iterateAll(cc context.Context, showSpinner bool, fn func(req *api.PaginationRequest) (*api.PaginationResponse, error)) error {
	term := ctx.Terminal(cc)
	pageReq := api.PaginationRequest{
		Limit: 100,
	}

	var spin *spinner.Spinner
	defer func() {
		if spin != nil {
			spin.Stop()
			term.Printf("\r")
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
			if spin == nil && showSpinner { // Start spinner on second page
				spin = spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
				spin.FinalMSG = "\r" + strings.Repeat(" ", 20) + "\r"
				spin.Suffix = " Fetching ..."
				spin.Start()
			}
		}

		if pageReq.Page == "" || cc.Err() != nil {
			break
		}
	}

	return nil
}
