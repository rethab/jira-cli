package jira

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUserSearch(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/3/user/assignable/search", r.URL.Path)

		if unexpectedStatusCode {
			w.WriteHeader(400)
		} else {
			assert.Equal(t, url.Values{
				"query":      []string{"doe"},
				"project":    []string{"TEST"},
				"issueKey":   []string{"ISSUE-1"},
				"startAt":    []string{"1"},
				"maxResults": []string{"5"},
				"accountId":  []string{"a123b"},
			}, r.URL.Query())

			resp, err := os.ReadFile("./testdata/users.json")
			assert.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write(resp)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.UserSearch(&UserSearchOptions{
		Query:      "doe",
		Project:    "TEST",
		IssueKey:   "ISSUE-1",
		AccountID:  "a123b",
		StartAt:    1,
		MaxResults: 5,
	})
	assert.NoError(t, err)

	expected := []*User{
		{
			AccountID:   "5fb82376aca10c006949f35b",
			Email:       "jane@domain.tld",
			Name:        "janedoe",
			DisplayName: "Jane Doe",
			Active:      true,
		},
		{
			AccountID:   "5fb82376aca10c006949f35c",
			Email:       "jon@domain.tld",
			DisplayName: "Jon Doe",
			Active:      false,
		},
	}
	assert.Equal(t, expected, actual)

	unexpectedStatusCode = true

	_, err = client.UserSearch(nil)
	assert.ErrorIs(t, err, ErrInvalidSearchOption)

	_, err = client.UserSearch(&UserSearchOptions{})
	assert.ErrorIs(t, err, ErrInvalidSearchOption)
}

func TestUserSearchV2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/2/user/assignable/search", r.URL.Path)

		assert.Equal(t, url.Values{
			"username":   []string{"doe"},
			"issueKey":   []string{"ISSUE-1"},
			"startAt":    []string{"1"},
			"maxResults": []string{"5"},
			"accountId":  []string{"a123b"},
		}, r.URL.Query())

		resp, err := os.ReadFile("./testdata/users.json")
		assert.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(resp)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.UserSearchV2(&UserSearchOptions{
		Query:      "doe",
		IssueKey:   "ISSUE-1",
		AccountID:  "a123b",
		StartAt:    1,
		MaxResults: 5,
	})
	assert.NoError(t, err)

	expected := []*User{
		{
			AccountID:   "5fb82376aca10c006949f35b",
			Email:       "jane@domain.tld",
			Name:        "janedoe",
			DisplayName: "Jane Doe",
			Active:      true,
		},
		{
			AccountID:   "5fb82376aca10c006949f35c",
			Email:       "jon@domain.tld",
			DisplayName: "Jon Doe",
			Active:      false,
		},
	}
	assert.Equal(t, expected, actual)

	actual, err = client.UserSearchV2(&UserSearchOptions{
		Query:      "doe",
		IssueKey:   "ISSUE-1",
		AccountID:  "a123b",
		StartAt:    1,
		MaxResults: 5,
	})
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestUserSearchViewIssue(t *testing.T) {
	cases := []struct {
		name   string
		search func(*Client, *UserSearchOptions) ([]*User, error)
		path   string
		opt    *UserSearchOptions
		want   url.Values
	}{
		{
			name:   "v3 by issue",
			search: (*Client).UserSearchViewIssue,
			path:   "/rest/api/3/user/viewissue/search",
			opt:    &UserSearchOptions{IssueKey: "ISSUE-1", Query: "jane doe", MaxResults: 10},
			want:   url.Values{"issueKey": {"ISSUE-1"}, "query": {"jane doe"}, "maxResults": {"10"}},
		},
		{
			name:   "v3 by project",
			search: (*Client).UserSearchViewIssue,
			path:   "/rest/api/3/user/viewissue/search",
			opt:    &UserSearchOptions{Project: "TEST", Query: "jane"},
			want:   url.Values{"projectKey": {"TEST"}, "query": {"jane"}},
		},
		{
			name:   "v2 by issue uses username",
			search: (*Client).UserSearchViewIssueV2,
			path:   "/rest/api/2/user/viewissue/search",
			opt:    &UserSearchOptions{IssueKey: "ISSUE-1", Query: "jane", MaxResults: 10},
			want:   url.Values{"issueKey": {"ISSUE-1"}, "username": {"jane"}, "maxResults": {"10"}},
		},
		{
			name:   "v2 by project",
			search: (*Client).UserSearchViewIssueV2,
			path:   "/rest/api/2/user/viewissue/search",
			opt:    &UserSearchOptions{Project: "TEST", Query: "jane"},
			want:   url.Values{"projectKey": {"TEST"}, "username": {"jane"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.path, r.URL.Path)
				assert.Equal(t, tc.want, r.URL.Query())

				resp, err := os.ReadFile("./testdata/users.json")
				assert.NoError(t, err)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_, _ = w.Write(resp)
			}))
			defer server.Close()

			client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

			users, err := tc.search(client, tc.opt)
			assert.NoError(t, err)
			assert.Len(t, users, 2)
			assert.Equal(t, "Jane Doe", users[0].DisplayName)
		})
	}
}

func TestUserSearchViewIssueErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	_, err := client.UserSearchViewIssue(nil)
	assert.ErrorIs(t, err, ErrInvalidSearchOption)

	_, err = client.UserSearchViewIssueV2(nil)
	assert.ErrorIs(t, err, ErrInvalidSearchOption)

	_, err = client.UserSearchViewIssue(&UserSearchOptions{IssueKey: "ISSUE-1", Query: "jane"})
	assertUnexpectedResponse(t, err)
}
