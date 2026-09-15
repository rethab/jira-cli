package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
