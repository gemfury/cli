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
func (c *Client) Versions(cc context.Context, pkg string, body *PaginationRequest) (*VersionsResponse, error) {
	req := c.newRequest(cc, "GET", "/packages/"+url.PathEscape(pkg)+"/versions?expand=package", true)

	if body != nil {
		c.prepareJSONBody(req, body)
	}

	resp := VersionsResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Versions)
	resp.Pagination = pagination

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
	Kind           string   `json:"kind"`
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
	ID        string           `json:"id"`
	Version   string           `json:"version"`
	Package   *Package         `json:"package,omitempty"`
	CreatedBy *AccountResponse `json:"created_by"`
	CreatedAt time.Time        `json:"created_at"`
}

func (v Version) DisplayCreatedBy() string {
	if a := v.CreatedBy; a != nil {
		return a.Name
	}
	return "N/A"
}
