package api

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Listing all versions for an account for backup purposes, etc
func (c *Client) DumpVersions(cc context.Context, body *PaginationRequest) (*VersionsResponse, error) {
	req := c.newRequest(cc, "GET", "/versions/$dump", true)

	if body != nil {
		c.prepareJSONBody(req, body)
	}

	resp := VersionsResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Versions)
	resp.Pagination = pagination

	return &resp, err
}

func (c *Client) DownloadVersion(cc context.Context, v *Version) (io.ReadCloser, int64, error) {
	if !strings.HasPrefix(v.DownloadURL, defaultEndpoint) {
		return nil, 0, fmt.Errorf("Download URL not compatible with API client")
	}

	path := strings.TrimPrefix(v.DownloadURL, defaultEndpoint)
	resp, err := c.newRequest(cc, "GET", path, true).doCommon()
	return resp.Body, resp.ContentLength, err
}
