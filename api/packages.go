package api

import (
	"context"
	"net/url"
	"time"
)

// Packages returns the details of the package listing
func (c *Client) Packages(cc context.Context, body *PaginationRequest) (*PackagesResponse, error) {
	req := c.newRequest(cc, "GET", "/packages", true)

	if body != nil {
		c.prepareJSONBody(req, body)
	}

	resp := PackagesResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Packages)
	resp.Pagination = pagination

	return &resp, err
}

// Versions returns the details of the versions listing for a package
func (c *Client) PackageVersions(cc context.Context, pkg string, body *PaginationRequest) (*VersionsResponse, error) {
	req := c.newRequest(cc, "GET", "/packages/"+url.PathEscape(pkg)+"/versions?expand=package", true)

	if body != nil {
		c.prepareJSONBody(req, body)
	}

	resp := VersionsResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Versions)
	resp.Pagination = pagination

	return &resp, err
}

// Versions returns the details of the versions listing for specified filters
func (c *Client) Versions(cc context.Context, filter url.Values, body *PaginationRequest) (*VersionsResponse, error) {
	req := c.newRequest(cc, "GET", "/versions?expand=package&"+filter.Encode(), true)

	if body != nil {
		c.prepareJSONBody(req, body)
	}

	resp := VersionsResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Versions)
	resp.Pagination = pagination

	return &resp, err
}

// Version returns the details of a specific version of a package
func (c *Client) Version(cc context.Context, pkg, ver string) (*Version, error) {
	path := "/packages/" + url.PathEscape(pkg) + "/versions/" + url.PathEscape(ver)
	req := c.newRequest(cc, "GET", path+"?expand=package", true)

	resp := Version{}
	err := req.doJSON(&resp)
	return &resp, err
}

// PackageResponse represents details from Packages API call
type PackagesResponse struct {
	Pagination *PaginationResponse
	Packages   []*Package
}

// VersionsResponse represents details from Versions API call
type VersionsResponse struct {
	Pagination *PaginationResponse
	Versions   []*Version
}

// Package represents Package JSON
type Package struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Kind           string   `json:"kind_key"`
	IsPrivate      bool     `json:"private"`
	LatestVersion  Version  `json:"latest_version"`
	ReleaseVersion *Version `json:"release_version"`
}

func (p Package) Privacy() string {
	if p.IsPrivate {
		return "private"
	}
	return "public"
}

func (p Package) DisplayVersion() string {
	if r := p.ReleaseVersion; r != nil {
		return r.Version
	}
	return "beta"
}

// VersionResponse represents Version JSON
type Version struct {
	ID          string           `json:"id"`
	Version     string           `json:"version"`
	Package     *Package         `json:"package,omitempty"`
	CreatedBy   *AccountResponse `json:"created_by"`
	CreatedAt   time.Time        `json:"created_at"`
	DownloadURL string           `json:"download_url"`
	Filename    string           `json:"filename"`
	Digests     VersionDigests   `json:"digests"`
}

// VersionDigests represents Version's digest field
type VersionDigests struct {
	SHA512 string `json:"sha512"`
	SHA256 string `json:"sha256"`
	SHA1   string `json:"sha1"`
	MD5    string `json:"md5"`
}

func (v Version) DisplayCreatedBy() string {
	if a := v.CreatedBy; a != nil {
		return a.Name
	}
	return "N/A"
}

func (v Version) Kind() string {
	if p := v.Package; p != nil && p.Kind != "" {
		return p.Kind
	}
	return "N/A"
}
