package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIssueWorklogs(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/rest/api/2/issue/TEST-1/worklog", r.URL.Path)
		assert.Equal(t, "startAt=10&maxResults=50", r.URL.RawQuery)

		if unexpectedStatusCode {
			w.WriteHeader(400)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		res, err := json.Marshal(&WorklogPage{
			StartAt:    10,
			MaxResults: 50,
			Total:      62,
			Worklogs: []Worklog{
				{
					ID:               "1",
					Author:           WorklogAuthor{Name: "Person A", Email: "a@example.com"},
					Comment:          "standup",
					Started:          "2026-01-15T11:17:18.000-0500",
					TimeSpent:        "15m",
					TimeSpentSeconds: 900,
				},
			},
		})
		assert.NoError(t, err)
		_, _ = w.Write(res)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	out, err := client.GetIssueWorklogs("TEST-1", 10, 50)
	assert.NoError(t, err)
	assert.Equal(t, 62, out.Total)
	assert.Len(t, out.Worklogs, 1)
	assert.Equal(t, "a@example.com", out.Worklogs[0].Author.Email)
	assert.Equal(t, int64(900), out.Worklogs[0].TimeSpentSeconds)

	unexpectedStatusCode = true

	_, err = client.GetIssueWorklogs("TEST-1", 10, 50)
	assertUnexpectedResponse(t, err)
}

func TestGetIssueWorklogsInWindow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/rest/api/2/issue/TEST-1/worklog", r.URL.Path)
		assert.Equal(t, "startedAfter=1000&startedBefore=2000&maxResults=5000", r.URL.RawQuery)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		res, err := json.Marshal(&WorklogPage{Total: 0, Worklogs: []Worklog{}})
		assert.NoError(t, err)
		_, _ = w.Write(res)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	out, err := client.GetIssueWorklogsInWindow("TEST-1", 1000, 2000)
	assert.NoError(t, err)
	assert.Equal(t, 0, out.Total)
}

func TestGetIssueWorklog(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/rest/api/2/issue/TEST-1/worklog/10042", r.URL.Path)

		if unexpectedStatusCode {
			w.WriteHeader(400)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"10042","comment":"standup","started":"2026-01-15T11:17:18.000-0500",
			"timeSpent":"15m","timeSpentSeconds":900,"visibility":{"type":"role","value":"Developers"}}`))
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	out, err := client.GetIssueWorklog("TEST-1", "10042")
	require.NoError(t, err)
	assert.Equal(t, "10042", out.ID)
	assert.Equal(t, "15m", out.TimeSpent)
	assert.Equal(t, &CommentVisibility{Type: "role", Value: "Developers"}, out.Visibility)

	unexpectedStatusCode = true

	_, err = client.GetIssueWorklog("TEST-1", "10042")
	assertUnexpectedResponse(t, err)
}

func TestUpdateIssueWorklog(t *testing.T) {
	tests := []struct {
		name          string
		req           WorklogUpdate
		newEstimate   string
		expectedBody  string
		expectedQuery url.Values
	}{
		{
			name:          "all fields",
			req:           WorklogUpdate{Started: "2022-01-01T01:02:02.000+0200", TimeSpent: "2h", Comment: "*new*"},
			expectedBody:  `{"started":"2022-01-01T01:02:02.000+0200","timeSpent":"2h","comment":"*new*"}`,
			expectedQuery: url.Values{},
		},
		{
			name:          "visibility is sent back",
			req:           WorklogUpdate{TimeSpent: "2h", Comment: "x", Visibility: &CommentVisibility{Type: "role", Value: "Developers"}},
			expectedBody:  `{"timeSpent":"2h","comment":"x","visibility":{"type":"role","value":"Developers"}}`,
			expectedQuery: url.Values{},
		},
		{
			name:          "new estimate is escaped",
			req:           WorklogUpdate{TimeSpent: "2h", Comment: "x"},
			newEstimate:   "1h 30m",
			expectedBody:  `{"timeSpent":"2h","comment":"x"}`,
			expectedQuery: url.Values{"adjustEstimate": {"new"}, "newEstimate": {"1h 30m"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var unexpectedStatusCode bool

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "/rest/api/2/issue/TEST-1/worklog/10042", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Accept"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, tc.expectedQuery, r.URL.Query())

				actual, _ := io.ReadAll(r.Body)
				assert.JSONEq(t, tc.expectedBody, string(actual))

				if unexpectedStatusCode {
					w.WriteHeader(400)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"id":"10042","timeSpent":"2h"}`))
			}))
			defer server.Close()

			client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

			out, err := client.UpdateIssueWorklog("TEST-1", "10042", tc.req, tc.newEstimate)
			require.NoError(t, err)
			assert.Equal(t, "10042", out.ID)
			assert.Equal(t, "2h", out.TimeSpent)

			unexpectedStatusCode = true

			_, err = client.UpdateIssueWorklog("TEST-1", "10042", tc.req, tc.newEstimate)
			assertUnexpectedResponse(t, err)
		})
	}
}

func TestDeleteIssueWorklog(t *testing.T) {
	var (
		unexpectedStatusCode bool
		expectedQuery        url.Values
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/rest/api/2/issue/TEST-1/worklog/10042", r.URL.Path)
		assert.Equal(t, expectedQuery, r.URL.Query())

		if unexpectedStatusCode {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	expectedQuery = url.Values{}
	assert.NoError(t, client.DeleteIssueWorklog("TEST-1", "10042", ""))

	expectedQuery = url.Values{"adjustEstimate": {"new"}, "newEstimate": {"1h 30m"}}
	assert.NoError(t, client.DeleteIssueWorklog("TEST-1", "10042", "1h 30m"))

	unexpectedStatusCode = true
	expectedQuery = url.Values{}

	err := client.DeleteIssueWorklog("TEST-1", "10042", "")
	assertUnexpectedResponse(t, err)
}
