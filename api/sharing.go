package api

import (
	"context"
	"net/url"
)

// AddCollaborator invites a collaborator via username or email
func (c *Client) AddCollaborator(cc context.Context, name, role string) error {
	path := "/collaborators/" + url.PathEscape(name)
	if role != "" {
		path = path + "?role=" + url.QueryEscape(role)
	}
	req := c.newRequest(cc, "PUT", path, true)
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
		if err := c.prepareJSONBody(req, body); err != nil {
			return nil, err
		}
	}

	resp := MembersResponse{}
	pagination, err := req.doPaginatedJSON(&resp.Members)
	resp.Pagination = pagination

	return &resp, err
}

// Collaborations returns the details of collaborations for the Viewer
func (c *Client) Collaborations(cc context.Context, body *PaginationRequest) (*MembersResponse, error) {
	req := c.newRequest(cc, "GET", "/collaborations", true)

	if body != nil {
		if err := c.prepareJSONBody(req, body); err != nil {
			return nil, err
		}
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

func (r *MembersResponse) Page() ([]*Member, *PaginationResponse) {
	return r.Members, r.Pagination
}

// Member represents Member JSON. The type is given for collaborations,
// and empty for members.
type Member struct {
	AccountBasic
	Type     string `json:"type"`
	Username string `json:"username"`
	Role     string `json:"role"`
}
