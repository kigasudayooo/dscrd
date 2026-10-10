package api

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

// DoMultipart invokes a Discord REST endpoint with a multipart/form-data body:
// a payload_json part plus one file attachment part named files[0]. The body is
// built once and replayed on retries so the boundary stays consistent.
func (c *Client) DoMultipart(method, path string, payloadJSON []byte, filePath string) ([]byte, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("%s %s: reading %s: %w", method, path, filePath, err)
	}

	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	if err := w.WriteField("payload_json", string(payloadJSON)); err != nil {
		return nil, err
	}
	part, err := w.CreateFormFile("files[0]", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	body := buf.Bytes()
	return c.roundTrip(method, path, path, w.FormDataContentType(), func() (io.Reader, error) {
		return bytes.NewReader(body), nil
	})
}
