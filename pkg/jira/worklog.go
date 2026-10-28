package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/rethab/jira-cli/pkg/md"
)

// WorklogAuthor holds the author of a worklog entry.
type WorklogAuthor struct {
	Name  string `json:"displayName"`
	Email string `json:"emailAddress"`
}

// Worklog holds a single worklog entry as returned by the Jira worklog endpoint.
type Worklog struct {
	ID               string        `json:"id"`
	Author           WorklogAuthor `json:"author"`
	Comment          string        `json:"comment"`
	Started          string        `json:"started"`
	TimeSpent        string        `json:"timeSpent"`
	TimeSpentSeconds int64         `json:"timeSpentSeconds"`

	Visibility *CommentVisibility `json:"visibility,omitempty"`
}

// WorklogUpdate is the body of a worklog create or update. Comment is wiki
// markup. Set either TimeSpent or TimeSpentSeconds: the formatted TimeSpent
// Jira returns is rounded to minutes, so sending it back would change a worklog
// that has seconds.
type WorklogUpdate struct {
	Started          string             `json:"started,omitempty"`
	TimeSpent        string             `json:"timeSpent,omitempty"`
	TimeSpentSeconds int64              `json:"timeSpentSeconds,omitempty"`
	Comment          string             `json:"comment"`
	Visibility       *CommentVisibility `json:"visibility,omitempty"`
}

// WorklogPage holds a page of worklog entries. StartAt, MaxResults and Total
// describe the full result set, not just the entries returned in Worklogs.
type WorklogPage struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Worklogs   []Worklog `json:"worklogs"`
}

// GetIssueWorklogs fetches a page of worklogs for an issue using the dedicated
// GET /issue/{key}/worklog endpoint. Unlike the worklog field embedded in the
// issue resource, this endpoint paginates properly instead of capping at 20
// entries. Entries are returned oldest first.
func (c *Client) GetIssueWorklogs(key string, startAt, maxResults uint) (*WorklogPage, error) {
	path := fmt.Sprintf("/issue/%s/worklog?startAt=%d&maxResults=%d", key, startAt, maxResults)
	return c.getIssueWorklogs(path)
}

// GetIssueWorklogsInWindow fetches every worklog for an issue whose started
// date falls within [afterMS, beforeMS), given as epoch milliseconds. The
// filtering happens server-side, which sidesteps pagination for a narrow
// window such as a single day.
func (c *Client) GetIssueWorklogsInWindow(key string, afterMS, beforeMS int64) (*WorklogPage, error) {
	path := fmt.Sprintf("/issue/%s/worklog?startedAfter=%d&startedBefore=%d&maxResults=5000", key, afterMS, beforeMS)
	return c.getIssueWorklogs(path)
}

func (c *Client) getIssueWorklogs(path string) (*WorklogPage, error) {
	res, err := c.GetV2(context.Background(), path, nil)
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

	var out WorklogPage
	err = json.NewDecoder(res.Body).Decode(&out)

	return &out, err
}

// AddIssueWorklog adds worklog to an issue using POST /issue/{key}/worklog endpoint.
// Leave param `started` empty to use the server's current datetime as start date.
func (c *Client) AddIssueWorklog(key, started, timeSpent, comment, newEstimate string) (*Worklog, error) {
	body, err := json.Marshal(&WorklogUpdate{
		Started:   started,
		TimeSpent: timeSpent,
		Comment:   md.ToJiraMD(comment),
	})
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/issue/%s/worklog%s", key, estimateQuery(newEstimate))
	return c.sendWorklog(http.MethodPost, path, body, http.StatusCreated)
}

// GetIssueWorklog fetches a single worklog using GET /issue/{key}/worklog/{id} endpoint.
func (c *Client) GetIssueWorklog(key, id string) (*Worklog, error) {
	path := fmt.Sprintf("/issue/%s/worklog/%s", key, id)
	return c.sendWorklog(http.MethodGet, path, nil, http.StatusOK)
}

// UpdateIssueWorklog replaces a worklog using PUT /issue/{key}/worklog/{id} endpoint.
//
// The endpoint overwrites the fields that are sent, so callers pass the
// existing values for everything they don't want to change, visibility
// included, or a restricted worklog would become visible to everyone.
func (c *Client) UpdateIssueWorklog(key, id string, req WorklogUpdate, newEstimate string) (*Worklog, error) {
	body, err := json.Marshal(&req)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/issue/%s/worklog/%s%s", key, id, estimateQuery(newEstimate))
	return c.sendWorklog(http.MethodPut, path, body, http.StatusOK)
}

// DeleteIssueWorklog deletes a worklog using DELETE /issue/{key}/worklog/{id} endpoint.
// Leave param `newEstimate` empty to let Jira adjust the remaining estimate itself.
func (c *Client) DeleteIssueWorklog(key, id, newEstimate string) error {
	path := fmt.Sprintf("/issue/%s/worklog/%s%s", key, id, estimateQuery(newEstimate))
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

// sendWorklog sends a request to the v2 api and decodes the worklog in the
// response, which must have the given status.
func (c *Client) sendWorklog(method, path string, body []byte, status int) (*Worklog, error) {
	res, err := c.request(context.Background(), method, c.server+baseURLv2+path, body, Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != status {
		return nil, formatUnexpectedResponse(res)
	}

	var out Worklog
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// estimateQuery returns the query string that sets the remaining estimate of
// the issue, or nothing to let Jira adjust it automatically. The estimate
// contains spaces, eg: 1h 30m, so it has to be escaped.
func estimateQuery(newEstimate string) string {
	if newEstimate == "" {
		return ""
	}
	return "?" + url.Values{"adjustEstimate": {"new"}, "newEstimate": {newEstimate}}.Encode()
}
