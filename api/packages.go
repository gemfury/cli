package api

import (
	"cmp"
	"context"
	"net/url"
	"time"
)

// Packages returns the details of the package listing
func (c *Client) Packages(cc context.Context, body *PaginationRequest) (*PackagesResponse, error) {
	req := c.newRequest(cc, "GET", "/packages", true)

	if body != nil {
		if err := c.prepareJSONBody(req, body); err != nil {
			return nil, err
		}
	}

	resp := PackagesResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Packages)
	resp.Pagination = pagination

	return &resp, err
}

// PackageVersions returns the details of the versions listing for a package
func (c *Client) PackageVersions(cc context.Context, pkg string, body *PaginationRequest) (*VersionsResponse, error) {
	req := c.newRequest(cc, "GET", "/packages/"+url.PathEscape(pkg)+"/versions?expand=package", true)

	if body != nil {
		if err := c.prepareJSONBody(req, body); err != nil {
			return nil, err
		}
	}

	resp := VersionsResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Versions)
	resp.Pagination = pagination
	resolveKinds(resp.Versions)

	return &resp, err
}

// Versions returns the details of the versions listing for specified filters
func (c *Client) Versions(cc context.Context, filter url.Values, body *PaginationRequest) (*VersionsResponse, error) {
	req := c.newRequest(cc, "GET", "/versions?expand=package&"+filter.Encode(), true)

	if body != nil {
		if err := c.prepareJSONBody(req, body); err != nil {
			return nil, err
		}
	}

	resp := VersionsResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Versions)
	resp.Pagination = pagination
	resolveKinds(resp.Versions)

	return &resp, err
}

// Version returns the details of a specific version of a package
func (c *Client) Version(cc context.Context, pkg, ver string) (*VersionFile, error) {
	path := "/packages/" + url.PathEscape(pkg) + "/versions/" + url.PathEscape(ver)
	req := c.newRequest(cc, "GET", path+"?expand=package", true)

	resp := VersionFile{}
	err := req.doJSON(&resp)
	resp.resolveKind()
	return &resp, err
}

// PackageResponse represents details from Packages API call
type PackagesResponse struct {
	Pagination *PaginationResponse
	Packages   []*Package
}

func (r *PackagesResponse) Page() ([]*Package, *PaginationResponse) {
	return r.Packages, r.Pagination
}

// VersionsResponse represents details from Versions API call
type VersionsResponse struct {
	Pagination *PaginationResponse
	Versions   []*Version
}

func (r *VersionsResponse) Page() ([]*Version, *PaginationResponse) {
	return r.Versions, r.Pagination
}

// Package represents Package JSON. The release version is null for a
// package with prereleases alone.
type Package struct {
	PackageBasic
	LatestVersion  VersionBasic  `json:"latest_version"`
	ReleaseVersion *VersionBasic `json:"release_version"`
}

// PackageBasic represents the Package JSON fields common to every package
type PackageBasic struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Kind         string `json:"kind_key"`
	IsPrivate    bool   `json:"private"`
	VersionCount int    `json:"version_count"`
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

// Version represents Version JSON
type Version struct {
	VersionBasic
	Kind      string         `json:"kind_key"`
	Digests   VersionDigests `json:"digests"`
	Filename  string         `json:"filename"`
	CreatedAt time.Time      `json:"created_at"`
	CreatedBy AccountBasic   `json:"created_by"`
	Package   PackageBasic   `json:"package"`
}

// VersionBasic represents the Version JSON fields common to every version
type VersionBasic struct {
	ID         string `json:"id"`
	Version    string `json:"version"`
	Prerelease bool   `json:"prerelease"`
}

// VersionDigests represents Version's digest field
type VersionDigests struct {
	MD5    string `json:"md5"`
	SHA1   string `json:"sha1"`
	SHA256 string `json:"sha256"`
	SHA512 string `json:"sha512"`
}

// resolveKind fills in the kind from the expanded package, for the
// responses of an older API that gives it there alone
func (v *Version) resolveKind() {
	v.Kind = cmp.Or(v.Kind, v.Package.Kind)
}

// resolveKinds does resolveKind for a listing of versions, or version files
func resolveKinds[V interface{ resolveKind() }](versions []V) {
	for _, v := range versions {
		v.resolveKind()
	}
}

func (v Version) DisplayCreatedBy() string {
	return cmp.Or(v.CreatedBy.Name, "N/A")
}

func (v Version) DisplayKind() string {
	return cmp.Or(v.Kind, "N/A")
}
