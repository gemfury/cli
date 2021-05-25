package api

import (
	"context"
	"net/url"
)

func (c *Client) AddCollaborator(cc context.Context, name string) error {
	url := c.urlFor("/1/collaborators/"+url.PathEscape(name), true)
	err := c.doJSON(cc, "PUT", url, nil)
	return err
}

func (c *Client) RemoveCollaborator(cc context.Context, name string) error {
	url := c.urlFor("/1/collaborators/"+url.PathEscape(name), true)
	err := c.doJSON(cc, "DELETE", url, nil)
	return err
}
