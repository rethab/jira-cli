package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ErrInvalidSearchOption denotes invalid search option was given.
var ErrInvalidSearchOption = fmt.Errorf("invalid search option")

// userSearchEndpoint is a user search path and the query parameter it takes the
// project key by, which differs between the endpoints.
type userSearchEndpoint struct {
	path, projectParam string
}

var (
	userAssignableSearch = userSearchEndpoint{path: "/user/assignable/search", projectParam: "project"}
	userViewIssueSearch  = userSearchEndpoint{path: "/user/viewissue/search", projectParam: "projectKey"}
)

// UserSearchOptions holds options to search for user.
type UserSearchOptions struct {
	Project    string
	IssueKey   string
	Query      string
	Username   string
	AccountID  string
	StartAt    int
	MaxResults int
}

// UserSearch search for user details using v3 version of the GET /user/assignable/search endpoint.
func (c *Client) UserSearch(opt *UserSearchOptions) ([]*User, error) {
	return c.userSearch(opt, apiVersion3)
}

// UserSearchV2 search for user details using v2 version of the GET /user/assignable/search endpoint.
func (c *Client) UserSearchV2(opt *UserSearchOptions) ([]*User, error) {
	// The `username` query param is deprecated since Jira API v2 and is not available in v3.
	// Since the` query` parameter doesn't seem to return expected results, we will use the
	// `username` param in call to v2. Chances are the `query` param may stop working in
	// later v2 updates so we might have to revisit this in the future. Note that the
	// `username` param is not as flexible as the `query` param available in v3.
	//
	// See https://github.com/rethab/jira-cli/issues/198
	if opt.Query != "" && opt.Username == "" {
		opt.Username = opt.Query
		opt.Query = ""
	}
	return c.userSearch(opt, apiVersion2)
}

// UserSearchViewIssue search for users that can view an issue or a project using v3 version of the
// GET /user/viewissue/search endpoint. Unlike the assignable search, it also finds users that can't
// be assigned, which is what a mention needs. Either IssueKey or Project must be set.
func (c *Client) UserSearchViewIssue(opt *UserSearchOptions) ([]*User, error) {
	return c.userSearchAt(opt, apiVersion3, userViewIssueSearch)
}

// UserSearchViewIssueV2 search for users that can view an issue or a project using v2 version of the
// GET /user/viewissue/search endpoint.
func (c *Client) UserSearchViewIssueV2(opt *UserSearchOptions) ([]*User, error) {
	if opt != nil {
		// See UserSearchV2 for why v2 takes `username` rather than `query`.
		cp := *opt
		if cp.Query != "" && cp.Username == "" {
			cp.Username = cp.Query
			cp.Query = ""
		}
		opt = &cp
	}
	return c.userSearchAt(opt, apiVersion2, userViewIssueSearch)
}

func (c *Client) userSearch(opt *UserSearchOptions, ver string) ([]*User, error) {
	return c.userSearchAt(opt, ver, userAssignableSearch)
}

func (c *Client) userSearchAt(opt *UserSearchOptions, ver string, endpoint userSearchEndpoint) ([]*User, error) {
	if opt == nil {
		return nil, ErrInvalidSearchOption
	}

	var (
		opts []string
		res  *http.Response
		err  error
	)

	if opt.Project != "" {
		opts = append(opts, fmt.Sprintf("%s=%s", endpoint.projectParam, opt.Project))
	}
	if opt.IssueKey != "" {
		opts = append(opts, fmt.Sprintf("issueKey=%s", opt.IssueKey))
	}
	if opt.Query != "" {
		opts = append(opts, fmt.Sprintf("query=%s", url.QueryEscape(opt.Query)))
	}
	if opt.Username != "" {
		opts = append(opts, fmt.Sprintf("username=%s", url.QueryEscape(opt.Username)))
	}
	if opt.AccountID != "" {
		opts = append(opts, fmt.Sprintf("accountId=%s", url.QueryEscape(opt.AccountID)))
	}
	if opt.StartAt != 0 {
		opts = append(opts, fmt.Sprintf("startAt=%d", opt.StartAt))
	}
	if opt.MaxResults != 0 {
		opts = append(opts, fmt.Sprintf("maxResults=%d", opt.MaxResults))
	}
	if len(opts) == 0 {
		return nil, ErrInvalidSearchOption
	}

	path := fmt.Sprintf("%s?%s", endpoint.path, strings.Join(opts, "&"))

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

	var out []*User
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
