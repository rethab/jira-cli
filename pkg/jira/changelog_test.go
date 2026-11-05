package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangelog(t *testing.T) {
	var (
		starts               []string
		omitIsLast           bool
		unexpectedStatusCode bool
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/rest/api/3/issue/TEST-1/changelog", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("maxResults"))

		if unexpectedStatusCode {
			w.WriteHeader(400)
			return
		}

		start := r.URL.Query().Get("startAt")
		starts = append(starts, start)

		page := map[string]any{"total": 2}
		switch start {
		case "0":
			page["values"] = []ChangelogHistory{{ID: "1", Created: "2026-10-01T14:03:00.000+0000"}}
			if !omitIsLast {
				page["isLast"] = false
			}
		case "1":
			page["values"] = []ChangelogHistory{{ID: "2", Items: []ChangelogItem{{Field: "status", FromString: "To Do", ToString: "Done"}}}}
			if !omitIsLast {
				page["isLast"] = true
			}
		default:
			t.Errorf("unexpected startAt %q", start)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		assert.NoError(t, json.NewEncoder(w).Encode(page))
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	t.Run("follows pages until isLast", func(t *testing.T) {
		starts = nil

		out, err := client.Changelog("TEST-1")
		require.NoError(t, err)

		assert.Equal(t, []string{"0", "1"}, starts)
		require.Len(t, out, 2)
		assert.Equal(t, "1", out[0].ID)
		assert.Equal(t, "2", out[1].ID)
		assert.Equal(t, "Done", out[1].Items[0].ToString)
	})

	t.Run("falls back to total when isLast is absent", func(t *testing.T) {
		starts = nil
		omitIsLast = true

		out, err := client.Changelog("TEST-1")
		require.NoError(t, err)

		assert.Equal(t, []string{"0", "1"}, starts)
		assert.Len(t, out, 2)
	})

	t.Run("unexpected status", func(t *testing.T) {
		unexpectedStatusCode = true

		_, err := client.Changelog("TEST-1")
		assertUnexpectedResponse(t, err)
	})
}

func TestChangelogV2(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/rest/api/2/issue/TEST-1", r.URL.Path)
		assert.Equal(t, "changelog", r.URL.Query().Get("expand"))

		if unexpectedStatusCode {
			w.WriteHeader(400)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"key":"TEST-1","changelog":{"histories":[
			{"id":"7","author":{"name":"jdoe","displayName":"Jane Doe"},"created":"2026-10-01T14:03:00.000+0000",
			 "items":[{"field":"status","fieldId":"status","from":"1","fromString":"To Do","to":"3","toString":"In Progress"}]}]}}`))
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	out, err := client.ChangelogV2("TEST-1")
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "Jane Doe", out[0].Author.DisplayName)
	assert.Equal(t, ChangelogItem{
		Field: "status", FieldID: "status", From: "1", FromString: "To Do", To: "3", ToString: "In Progress",
	}, out[0].Items[0])

	unexpectedStatusCode = true

	_, err = client.ChangelogV2("TEST-1")
	assertUnexpectedResponse(t, err)
}
