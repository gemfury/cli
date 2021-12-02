package api

import (
	"context"
	"io"
	"mime/multipart"
)

// PushPkg uploads a single package file to the current account
func (c *Client) PushPkg(cc context.Context, filename string, isPublic bool, r io.Reader) error {
	bodyR, bodyW := io.Pipe()
	writer := multipart.NewWriter(bodyW)

	go func() {
		// Public vs. private
		if isPublic {
			writer.WriteField("public", "true")
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

	req := c.newPushRequest(cc, "POST", "/uploads", true)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Request.Body = bodyR

	err := req.doJSON(nil)
	return err
}
