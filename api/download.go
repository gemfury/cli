package api

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// Listing all versions for an account for backup purposes, etc
func (c *Client) DumpVersions(cc context.Context, body *PaginationRequest, kindFilter string) (*VersionsResponse, error) {
	path := "/versions/$dump"
	if kindFilter != "" {
		path = path + "?kind=" + url.QueryEscape(kindFilter)
	}

	req := c.newRequest(cc, "GET", path, true)

	if body != nil {
		c.prepareJSONBody(req, body)
	}

	resp := VersionsResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Versions)
	resp.Pagination = pagination

	return &resp, err
}

// DownloadVersion uses the "download_url" field to download the Version file
func (c *Client) DownloadVersion(cc context.Context, v *Version) (io.ReadCloser, int64, error) {
	if !strings.HasPrefix(v.DownloadURL, c.Endpoint) {
		return nil, 0, fmt.Errorf("Download URL not compatible with API client")
	}

	path := strings.TrimPrefix(v.DownloadURL, defaultEndpoint)
	resp, err := c.newRequest(cc, "GET", path, true).doCommon()
	return resp.Body, resp.ContentLength, err
}
