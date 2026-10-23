package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

// Attachment is a file attached to an issue.
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Author   User   `json:"author"`
	Created  string `json:"created"`
	Content  string `json:"content"`
}

// GetIssueAttachments fetches the attachments of an issue using the
// attachment field of GET /issue/{key} endpoint. Jira has no endpoint that
// lists the attachments of an issue directly.
func (c *Client) GetIssueAttachments(key string) ([]Attachment, error) {
	var out struct {
		Fields struct {
			Attachment []Attachment `json:"attachment"`
		} `json:"fields"`
	}

	if err := c.getJSON(baseURLv2, fmt.Sprintf("/issue/%s?fields=attachment", key), &out); err != nil {
		return nil, err
	}
	return out.Fields.Attachment, nil
}

// AddIssueAttachment uploads a file to an issue using POST /issue/{key}/attachments endpoint.
func (c *Client) AddIssueAttachment(key, filename string, content io.Reader) ([]Attachment, error) {
	var body bytes.Buffer

	mw := multipart.NewWriter(&body)

	part, err := mw.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, content); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/issue/%s/attachments", key)
	res, err := c.PostV2(context.Background(), path, body.Bytes(), Header{
		"Accept":            "application/json",
		"Content-Type":      mw.FormDataContentType(),
		"X-Atlassian-Token": "no-check",
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, formatUnexpectedResponse(res)
	}

	var out []Attachment
	err = json.NewDecoder(res.Body).Decode(&out)

	return out, err
}

// DownloadAttachment streams the content of an attachment to w.
//
// The download link Jira reports differs by installation: Cloud points at
// /rest/api/{v}/attachment/content/{id} on the site, and Server/Data Center at
// /secure/attachment/{id}/{name} below its base URL. Only the part from there
// on is used, re-based onto the configured server, so credentials only ever go
// to that server, whatever host the link names. That also makes Cloud links
// work through the Atlassian API gateway (scoped tokens), and Server links when
// Jira's base URL differs from the configured one. Cloud then redirects to its
// media host, and net/http drops the Authorization header on that redirect.
func (c *Client) DownloadAttachment(a Attachment, w io.Writer) error {
	link, err := c.attachmentURL(a)
	if err != nil {
		return err
	}

	res, err := c.send(context.Background(), http.MethodGet, link, nil, nil)
	if err != nil {
		return err
	}
	if res == nil {
		return ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return formatUnexpectedResponse(res)
	}

	_, err = io.Copy(w, res.Body)

	return err
}

func (c *Client) attachmentURL(a Attachment) (string, error) {
	u, err := url.Parse(a.Content)
	if err != nil || a.Content == "" {
		return "", fmt.Errorf("attachment %q has no usable download link", a.Filename)
	}

	p := u.EscapedPath()
	for _, marker := range []string{"/rest/api/", "/secure/attachment/"} {
		if i := strings.Index(p, marker); i >= 0 {
			link := c.server + p[i:]
			if u.RawQuery != "" {
				link += "?" + u.RawQuery
			}
			return link, nil
		}
	}
	return "", fmt.Errorf("attachment %q has an unrecognized download link %q", a.Filename, a.Content)
}
