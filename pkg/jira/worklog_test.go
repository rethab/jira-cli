package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
