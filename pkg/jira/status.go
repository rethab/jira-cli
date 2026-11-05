package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// Status is an issue status as returned by the GET /status endpoint.
type Status struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StatusCategory struct {
		Key string `json:"key"`
	} `json:"statusCategory"`
}

// Statuses fetches all statuses using v2 version of the GET /status endpoint, which exists on
// Cloud and Server/Data Center and doesn't need admin permissions.
func (c *Client) Statuses() ([]Status, error) {
	res, err := c.GetV2(context.Background(), "/status", nil)
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

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var out []Status
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}

	return out, nil
}
