package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	// commentInternalProperty is the entity property Jira Service Management
	// uses to mark a comment as internal.
	commentInternalProperty = "sd.public.comment"

	commentPageSize = 100
)

// CommentProperty is an entity property attached to a comment.
type CommentProperty struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// CommentVisibility restricts a comment to a group or project role.
type CommentVisibility struct {
	Type       string `json:"type"`
	Value      string `json:"value"`
	Identifier string `json:"identifier,omitempty"`
}

// Comment is a single issue comment as returned by the v2 comment endpoints,
// where the body is Jira wiki markup.
type Comment struct {
	ID           string             `json:"id"`
	Author       User               `json:"author"`
	UpdateAuthor User               `json:"updateAuthor"`
	Body         string             `json:"body"`
	Created      string             `json:"created"`
	Updated      string             `json:"updated"`
	Visibility   *CommentVisibility `json:"visibility,omitempty"`
	Properties   []CommentProperty  `json:"properties,omitempty"`
}

type commentPage struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Comments   []Comment `json:"comments"`
}

// GetIssueComments fetches every comment of an issue, oldest first, using the
// paginated GET /issue/{key}/comment endpoint. The comment field embedded in
// the issue resource is capped, so it can't be used to list them all.
func (c *Client) GetIssueComments(key string) ([]Comment, error) {
	var comments []Comment

	for {
		path := fmt.Sprintf("/issue/%s/comment?startAt=%d&maxResults=%d", key, len(comments), commentPageSize)

		var page commentPage
		if err := c.getJSON(path, &page); err != nil {
			return nil, err
		}

		comments = append(comments, page.Comments...)

		// An empty page ends the loop even if the server's total is off, so
		// a misbehaving server can't make us spin.
		if len(page.Comments) == 0 || len(comments) >= page.Total {
			return comments, nil
		}
	}
}

// GetIssueComment fetches a single comment, including its properties, using
// GET /issue/{key}/comment/{id} endpoint.
func (c *Client) GetIssueComment(key, id string) (*Comment, error) {
	var comment Comment

	path := fmt.Sprintf("/issue/%s/comment/%s?expand=properties", key, id)
	if err := c.getJSON(path, &comment); err != nil {
		return nil, err
	}
	return &comment, nil
}

type issueCommentEditRequest struct {
	Body       string             `json:"body"`
	Visibility *CommentVisibility `json:"visibility,omitempty"`
	Properties []CommentProperty  `json:"properties,omitempty"`
}

// EditIssueComment replaces the body of a comment using PUT /issue/{key}/comment/{id} endpoint.
//
// The comment is fetched first and its visibility and internal flag are sent
// back with the new body. A PUT that omits them can reset them, which would
// publish an internal comment.
func (c *Client) EditIssueComment(key, id, comment string) error {
	existing, err := c.GetIssueComment(key, id)
	if err != nil {
		return err
	}

	req := issueCommentEditRequest{
		Body:       c.ToJiraMD(comment),
		Visibility: existing.Visibility,
	}
	for _, p := range existing.Properties {
		if p.Key == commentInternalProperty {
			req.Properties = append(req.Properties, p)
		}
	}

	body, err := json.Marshal(&req)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/issue/%s/comment/%s", key, id)
	res, err := c.PutV2(context.Background(), path, body, Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
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
	return nil
}

// DeleteIssueComment deletes a comment using DELETE /issue/{key}/comment/{id} endpoint.
func (c *Client) DeleteIssueComment(key, id string) error {
	path := fmt.Sprintf("/issue/%s/comment/%s", key, id)
	res, err := c.DeleteV2(context.Background(), path, nil)
	if err != nil {
		return err
	}
	if res == nil {
		return ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		return formatUnexpectedResponse(res)
	}
	return nil
}

// getJSON sends a GET request to the v2 api and decodes a 200 response into out.
func (c *Client) getJSON(path string, out any) error {
	res, err := c.GetV2(context.Background(), path, Header{"Accept": "application/json"})
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
	return json.NewDecoder(res.Body).Decode(out)
}
