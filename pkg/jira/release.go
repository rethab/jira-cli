package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// VersionCreateRequest holds request data for creating a version.
type VersionCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Project     string `json:"project,omitempty"`
	ProjectID   int64  `json:"projectId,omitempty"`
	Archived    bool   `json:"archived"`
	Released    bool   `json:"released"`
	ReleaseDate string `json:"releaseDate,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
}

// Release fetches response from /project/{projectIdOrKey}/version endpoint.
func (c *Client) Release(project string) ([]*ProjectVersion, error) {
	path := fmt.Sprintf("/project/%s/versions", project)
	res, err := c.Get(context.Background(), path, nil)
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

	var out []*ProjectVersion

	err = json.NewDecoder(res.Body).Decode(&out)

	return out, err
}

// CreateVersion creates a project version using POST /version endpoint.
func (c *Client) CreateVersion(req *VersionCreateRequest) (*ProjectVersion, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	res, err := c.PostV2(context.Background(), "/version", body, Header{
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

	if res.StatusCode != http.StatusCreated {
		return nil, formatUnexpectedResponse(res)
	}

	var out ProjectVersion

	err = json.NewDecoder(res.Body).Decode(&out)

	return &out, err
}
