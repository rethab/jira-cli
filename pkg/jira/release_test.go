package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReleases(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/3/project/1000/versions", r.URL.Path)

		if unexpectedStatusCode {
			w.WriteHeader(400)
		} else {

			resp, err := os.ReadFile("./testdata/releases.json")
			assert.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write(resp)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.Release("1000")
	assert.NoError(t, err)

	expected := []*ProjectVersion{
		{
			ID:          "1000",
			Description: "Release A",
			Name:        "First",
			Archived:    false,
			Released:    true,
			ProjectID:   1000,
		},
		{
			ID:          "1001",
			Description: "Release B",
			Name:        "Second",
			Archived:    false,
			Released:    false,
			ProjectID:   1000,
		},
		{
			ID:          "1002",
			Description: "Release C",
			Name:        "Third",
			Archived:    true,
			Released:    false,
			ProjectID:   1000,
		},
	}
	assert.Equal(t, expected, actual)

	unexpectedStatusCode = true

	_, err = client.Release("1000")
	assertUnexpectedResponse(t, err)
}

func TestCreateVersion(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/rest/api/2/version", r.URL.Path)

		if unexpectedStatusCode {
			w.WriteHeader(400)
			return
		}

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		var got map[string]any
		assert.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, map[string]any{
			"name":        "v1.0",
			"description": "First release",
			"project":     "TEST",
			"startDate":   "2024-01-01",
			"releaseDate": "2024-02-01",
			"released":    true,
			"archived":    false,
		}, got)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"10001","name":"v1.0","description":"First release","projectId":1000,"released":true,"archived":false}`))
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.CreateVersion(&VersionCreateRequest{
		Name:        "v1.0",
		Description: "First release",
		Project:     "TEST",
		StartDate:   "2024-01-01",
		ReleaseDate: "2024-02-01",
		Released:    true,
	})
	assert.NoError(t, err)
	assert.Equal(t, &ProjectVersion{
		ID:          "10001",
		Name:        "v1.0",
		Description: "First release",
		ProjectID:   1000,
		Released:    true,
	}, actual)

	unexpectedStatusCode = true

	_, err = client.CreateVersion(&VersionCreateRequest{Name: "v1.0", Project: "TEST"})
	assertUnexpectedResponse(t, err)
}
