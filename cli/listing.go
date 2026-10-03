package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"context"
	"fmt"
)

// fetchAll collects every page of a listing from fetch: a method of the
// API client, or a closure around one. A spinner shows from the second
// page on.
func fetchAll[T any, R api.Paged[T]](cc context.Context, fetch func(context.Context, *api.PaginationRequest) (R, error)) ([]T, error) {
	term := ctx.Terminal(cc)
	var stopSpinner func()
	defer func() {
		if stopSpinner != nil {
			stopSpinner()
		}
	}()

	var items []T
	err := iterateAll(cc, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
		if pageReq.Page != "" && stopSpinner == nil {
			stopSpinner = term.Spin(" Fetching ...")
		}

		resp, err := fetch(cc, pageReq)
		if err != nil {
			return nil, err
		}

		page, pagination := resp.Page()
		items = append(items, page...)
		return pagination, nil
	})
	return items, err
}

// printListing prints a listing as JSON with --json, or else as text by
// render, or the empty message when there is nothing to print. An error
// is returned as it is: with --json nothing is printed then, and
// otherwise what was listed before it.
func printListing[T any](cmd *cobra.Command, items []T, err error, empty string, render func(term terminal.Terminal)) error {
	term := ctx.Terminal(cmd.Context())
	if printsJSON(cmd) {
		if err != nil {
			return err
		}
		if items == nil {
			items = []T{} // Printed as [], not null
		}
		return termPrintJSON(term, items)
	}

	if len(items) == 0 {
		if err == nil {
			term.Infof("%s\n", empty)
		}
		return err
	}

	render(term)
	return err
}

// iterateAll requests every page of a listing from fn, until there is
// no next page, or a page repeats
func iterateAll(cc context.Context, fn func(req *api.PaginationRequest) (*api.PaginationResponse, error)) error {
	pageReq := api.PaginationRequest{
		Limit: 100,
	}

	seenCursors := map[string]struct{}{}

	for {
		pageResp, err := fn(&pageReq)
		if err != nil {
			return err
		}

		pageReq.Page = ""
		if pageResp != nil {
			pageReq.Page = pageResp.NextPageCursor()
		}

		// A cancelled listing is incomplete, and must not pass for success
		if err := cc.Err(); err != nil {
			return err
		}

		if pageReq.Page == "" {
			return nil
		}

		if _, ok := seenCursors[pageReq.Page]; ok {
			return fmt.Errorf("Repeated pagination cursor: %q", pageReq.Page)
		}
		seenCursors[pageReq.Page] = struct{}{}
	}
}
