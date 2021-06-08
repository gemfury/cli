package api

import (
	"context"
	"net/url"
)

// AddCollaborator invites a collaborator via username or email
func (c *Client) AddCollaborator(cc context.Context, name string) error {
	req := c.newRequest(cc, "PUT", "/collaborators/"+url.PathEscape(name), true)
	return req.doJSON(nil)
}

// RemoveCollaborator removes a collaborator via username or email
func (c *Client) RemoveCollaborator(cc context.Context, name string) error {
	req := c.newRequest(cc, "DELETE", "/collaborators/"+url.PathEscape(name), true)
	return req.doJSON(nil)
}

// Members returns the details of the collaborator listing
func (c *Client) Members(cc context.Context, body *PaginationRequest) (*MembersResponse, error) {
	req := c.newRequest(cc, "GET", "/members", true)

	if body != nil {
		c.prepareJSONBody(req, body)
	}

	resp := MembersResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Members)
	resp.Pagination = pagination

	return &resp, err
}

// MembersResponse represents details from Members API call
type MembersResponse struct {
	Pagination *PaginationResponse
	Members    []*Member
}

// Member represents Member JSON
type Member struct {
	Role string `json:"role"`
	AccountResponse
}
