package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rethab/jira-cli/pkg/adf"
)

// GetIssueDescription fetches the ADF description via GET /rest/api/3/issue/{key}?fields=description.
// It returns nil for an empty description and an error for one that isn't valid ADF.
func (c *Client) GetIssueDescription(key string) (*adf.ADF, error) {
	raw, err := c.getDescription(key, apiVersion3)
	if err != nil {
		return nil, err
	}
	if isJSONNull(raw) {
		return nil, nil
	}

	// Unlike ifaceToADF this must not turn malformed data into nil: callers
	// treat nil as an empty description and would overwrite the original.
	var doc adf.ADF
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("unable to decode the description of %s: %w", key, err)
	}
	if doc.DocType != "doc" {
		return nil, fmt.Errorf("the description of %s is not an Atlassian document", key)
	}

	return &doc, nil
}

// GetIssueDescriptionV2 fetches the wiki-markup description via v2; "" when empty.
func (c *Client) GetIssueDescriptionV2(key string) (string, error) {
	raw, err := c.getDescription(key, apiVersion2)
	if err != nil {
		return "", err
	}
	if isJSONNull(raw) {
		return "", nil
	}

	var wiki string
	if err := json.Unmarshal(raw, &wiki); err != nil {
		return "", fmt.Errorf("unable to decode the description of %s: %w", key, err)
	}

	return wiki, nil
}

// SetIssueDescription replaces the description via PUT /rest/api/3/issue/{key}.
func (c *Client) SetIssueDescription(key string, doc *adf.ADF, skipNotify bool) error {
	return c.setDescription(key, doc, skipNotify, apiVersion3)
}

// SetIssueDescriptionV2 replaces the description via PUT /rest/api/2/issue/{key}.
func (c *Client) SetIssueDescriptionV2(key, wiki string, skipNotify bool) error {
	return c.setDescription(key, wiki, skipNotify, apiVersion2)
}

func (c *Client) getDescription(key, ver string) (json.RawMessage, error) {
	path := fmt.Sprintf("/issue/%s?fields=description", key)
	headers := Header{"Accept": "application/json"}

	var (
		res *http.Response
		err error
	)
	if ver == apiVersion2 {
		res, err = c.GetV2(context.Background(), path, headers)
	} else {
		res, err = c.Get(context.Background(), path, headers)
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

	var out struct {
		Fields struct {
			Description json.RawMessage `json:"description"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out.Fields.Description, nil
}

func (c *Client) setDescription(key string, description any, skipNotify bool, ver string) error {
	body, err := json.Marshal(map[string]any{"fields": map[string]any{"description": description}})
	if err != nil {
		return err
	}

	endpoint := "/issue/" + key
	if skipNotify {
		endpoint += "?notifyUsers=false"
	}
	headers := Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}

	var res *http.Response
	if ver == apiVersion2 {
		res, err = c.PutV2(context.Background(), endpoint, body, headers)
	} else {
		res, err = c.Put(context.Background(), endpoint, body, headers)
	}
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

// A missing field leaves the raw message empty.
func isJSONNull(raw json.RawMessage) bool {
	t := bytes.TrimSpace(raw)
	return len(t) == 0 || bytes.Equal(t, []byte("null"))
}
