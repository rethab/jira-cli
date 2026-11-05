package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const changelogPageSize = 100

// ChangelogItem is a single field change within a changelog history entry.
type ChangelogItem struct {
	Field      string `json:"field"`
	FieldID    string `json:"fieldId"`
	FieldType  string `json:"fieldtype"`
	From       string `json:"from"`
	FromString string `json:"fromString"`
	To         string `json:"to"`
	ToString   string `json:"toString"`
}

// ChangelogHistory is one change of an issue, which can touch several fields at once.
type ChangelogHistory struct {
	ID      string          `json:"id"`
	Author  User            `json:"author"`
	Created string          `json:"created"`
	Items   []ChangelogItem `json:"items"`
}

type changelogPage struct {
	Total  int                `json:"total"`
	IsLast *bool              `json:"isLast"`
	Values []ChangelogHistory `json:"values"`
}

// Changelog fetches all changelog histories of an issue, oldest first, using v3 version of
// the GET /issue/{key}/changelog endpoint.
func (c *Client) Changelog(key string) ([]ChangelogHistory, error) {
	var all []ChangelogHistory

	for {
		page, err := c.changelogPage(key, len(all))
		if err != nil {
			return nil, err
		}

		all = append(all, page.Values...)

		// "isLast" is missing on some responses, so fall back to the total.
		last := len(page.Values) == 0 || len(all) >= page.Total
		if page.IsLast != nil {
			last = *page.IsLast || len(page.Values) == 0
		}
		if last {
			return all, nil
		}
	}
}

// ChangelogV2 fetches the changelog histories of an issue using v2 version of the GET /issue/{key} endpoint.
// The paged changelog endpoint doesn't exist on all Server/Data Center versions, and the expanded
// issue carries the whole changelog.
func (c *Client) ChangelogV2(key string) ([]ChangelogHistory, error) {
	path := fmt.Sprintf("/issue/%s?expand=changelog&fields=summary", key)

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

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var out struct {
		Changelog struct {
			Histories []ChangelogHistory `json:"histories"`
		} `json:"changelog"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}

	return out.Changelog.Histories, nil
}

func (c *Client) changelogPage(key string, startAt int) (*changelogPage, error) {
	path := fmt.Sprintf("/issue/%s/changelog?startAt=%d&maxResults=%d", key, startAt, changelogPageSize)

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

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var page changelogPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}

	return &page, nil
}
