package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// TransitionRequest struct holds request data for issue transition request.
type TransitionRequest struct {
	Update     *TransitionRequestUpdate `json:"update,omitempty"`
	Fields     *TransitionRequestFields `json:"fields,omitempty"`
	Transition *TransitionRequestData   `json:"transition"`
}

// TransitionRequestUpdate struct holds a list of operations to perform on the issue screen field.
type TransitionRequestUpdate struct {
	Comment []struct {
		Add struct {
			Body string `json:"body"`
		} `json:"add"`
	} `json:"comment,omitempty"`
}

// TransitionRequestFields struct holds a list of issue screen fields to update along with sub-fields.
type TransitionRequestFields struct {
	Assignee *struct {
		Name string `json:"name"`
	} `json:"assignee,omitempty"`
	Resolution *struct {
		Name string `json:"name"`
	} `json:"resolution,omitempty"`
}

// TransitionRequestData is a transition request data.
type TransitionRequestData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type transitionResponse struct {
	Expand      string        `json:"expand"`
	Transitions []*Transition `json:"transitions"`
}

// Transitions fetches valid transitions for an issue using v3 version of the GET /issue/{key}/transitions endpoint.
func (c *Client) Transitions(key string) ([]*Transition, error) {
	return c.transitions(key, apiVersion3, false)
}

// TransitionsV2 fetches valid transitions for an issue using v2 version of the GET /issue/{key}/transitions endpoint.
func (c *Client) TransitionsV2(key string) ([]*Transition, error) {
	return c.transitions(key, apiVersion2, false)
}

// TransitionsRaw fetches valid transitions same as Transitions but returns the raw API response body string.
func (c *Client) TransitionsRaw(key string) (string, error) {
	body, err := c.transitionsBody(key, apiVersion3, false)
	return string(body), err
}

// TransitionsV2Raw fetches valid transitions same as TransitionsV2 but returns the raw API response body string.
func (c *Client) TransitionsV2Raw(key string) (string, error) {
	body, err := c.transitionsBody(key, apiVersion2, false)
	return string(body), err
}

// TransitionsWithFields is Transitions with the fields of each transition's screen expanded.
func (c *Client) TransitionsWithFields(key string) ([]*Transition, error) {
	return c.transitions(key, apiVersion3, true)
}

// TransitionsV2WithFields is TransitionsV2 with the fields of each transition's screen expanded.
func (c *Client) TransitionsV2WithFields(key string) ([]*Transition, error) {
	return c.transitions(key, apiVersion2, true)
}

// TransitionsWithFieldsRaw is TransitionsRaw with the fields of each transition's screen expanded.
func (c *Client) TransitionsWithFieldsRaw(key string) (string, error) {
	body, err := c.transitionsBody(key, apiVersion3, true)
	return string(body), err
}

// TransitionsV2WithFieldsRaw is TransitionsV2Raw with the fields of each transition's screen expanded.
func (c *Client) TransitionsV2WithFieldsRaw(key string) (string, error) {
	body, err := c.transitionsBody(key, apiVersion2, true)
	return string(body), err
}

// AvailableTransitions drops the transitions Jira reports as unavailable. Jira API v2 doesn't return
// "isAvailable", so only Cloud transitions can be filtered.
func AvailableTransitions(all []*Transition, installation string) []*Transition {
	out := make([]*Transition, 0, len(all))
	for _, t := range all {
		if installation == InstallationTypeCloud && !t.IsAvailable {
			continue
		}
		out = append(out, t)
	}
	return out
}

func (c *Client) transitions(key, ver string, withFields bool) ([]*Transition, error) {
	body, err := c.transitionsBody(key, ver, withFields)
	if err != nil {
		return nil, err
	}

	var out transitionResponse

	err = json.Unmarshal(body, &out)

	return out.Transitions, err
}

func (c *Client) transitionsBody(key, ver string, withFields bool) ([]byte, error) {
	path := fmt.Sprintf("/issue/%s/transitions", key)
	if withFields {
		path += "?expand=transitions.fields"
	}

	var (
		res *http.Response
		err error
	)

	switch ver {
	case apiVersion2:
		res, err = c.GetV2(context.Background(), path, nil)
	default:
		res, err = c.Get(context.Background(), path, nil)
	}

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

	return io.ReadAll(res.Body)
}

// Transition moves issue from one state to another using POST /issue/{key}/transitions endpoint.
func (c *Client) Transition(key string, data *TransitionRequest) (int, error) {
	body, err := json.Marshal(&data)
	if err != nil {
		return 0, err
	}

	path := fmt.Sprintf("/issue/%s/transitions", key)

	res, err := c.PostV2(context.Background(), path, body, Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	if err != nil {
		return 0, err
	}
	if res == nil {
		return 0, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		return res.StatusCode, formatUnexpectedResponse(res)
	}
	return res.StatusCode, nil
}
