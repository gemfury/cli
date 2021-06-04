package api

import (
	"github.com/tomnomnom/linkheader"

	"net/http"
	"net/url"
)

// PaginationRequest for requesting paginated
type PaginationRequest struct {
	Limit int    `json:"limit,omitempty"`
	Page  string `json:"page,omitempty"`
}

// PaginationResponse for pagination metadata from API headers
type PaginationResponse struct {
	linkheader.Links
}

// parsePagination extracts header information into PaginationResponse
func parsePagination(resp *http.Response) *PaginationResponse {
	links := linkheader.Parse(resp.Header.Get("Link"))
	if links == nil {
		return nil
	}

	return &PaginationResponse{
		Links: links,
	}
}

// nextPageURL transforms URL to next page, or returns nil if no more pages
func (r PaginationResponse) NextPageCursor() string {
	for _, link := range r.Links.FilterByRel("next") {
		if u, err := url.Parse(link.URL); err == nil {
			return u.Query().Get("page")
		}
	}

	return ""
}
