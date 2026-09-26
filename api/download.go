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

// DownloadVersion uses the "download_url" field to download the Version file.
// The URL must be served by this client's API endpoint, since the request is
// authenticated with the client's token.
func (c *Client) DownloadVersion(cc context.Context, v *Version) (io.ReadCloser, int64, error) {
	path, ok := strings.CutPrefix(v.DownloadURL, c.Endpoint)
	if !ok || !strings.HasPrefix(path, "/") {
		return nil, 0, fmt.Errorf("Download URL %q is not served by %s", v.DownloadURL, c.Endpoint)
	}

	resp, err := c.newRequest(cc, "GET", path, true).doCommon()
	if err != nil {
		return nil, 0, err
	}

	return resp.Body, resp.ContentLength, nil
}
