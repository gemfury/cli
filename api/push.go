package api

import (
	"context"
	"io"
	"mime/multipart"
)

// PushPkg uploads a single package file to the current account
func (c *Client) PushPkg(cc context.Context, filename string, isPublic bool, r io.Reader) error {
	req := c.newPushRequest(cc, "POST", "/uploads", true)
	if req.err != nil {
		return req.err
	}

	bodyR, bodyW := io.Pipe()
	writer := multipart.NewWriter(bodyW)

	go func() {
		// Public vs. private
		if isPublic {
			if err := writer.WriteField("public", "true"); err != nil {
				bodyW.CloseWithError(err)
				return
			}
		}

		// Stream file content
		ff, err := writer.CreateFormFile("file", filename)
		if err != nil {
			bodyW.CloseWithError(err)
			return
		}

		_, err = io.Copy(ff, r)
		if err != nil {
			bodyW.CloseWithError(err)
			return
		}

		err = writer.Close()
		bodyW.CloseWithError(err)
	}()

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Request.Body = bodyR

	// Unblocks the writer above when the request ends before reading the body
	defer bodyR.Close()

	return req.doJSON(nil)
}
