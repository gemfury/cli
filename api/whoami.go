package api

import (
	"context"
)

type WhoAmIResponse struct {
	ID       string `json:id`
	Name     string `json:name`
	Username string `json:username`
}

func (c *Client) WhoAmI(cc context.Context) (*WhoAmIResponse, error) {
	url, resp := c.urlFor("/1/users/me", false), WhoAmIResponse{}
	err := c.doJSON(cc, "GET", url, &resp)
	return &resp, err
}
