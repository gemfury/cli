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
