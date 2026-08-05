package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func TestExists(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "it returns false for empty file",
			input:    "",
			expected: false,
		},
		{
			name:     "it returns false if file doesn't exist",
			input:    "invalid.txt",
			expected: false,
		},
		{
			name:     "it returns true if the file exist",
			input:    "/testdata/empty.txt",
			expected: true,
		},
	}

	cwd, err := os.Getwd()
	assert.NoError(t, err)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := tc.input
			if path != "" {
				path = cwd + tc.input
			}

			assert.Equal(t, tc.expected, Exists(path))
		})
	}
}

func TestCreate(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)

	file := cwd + "/testdata/.tmp/.jira.yml"

	// case: file doesn't exist
	assert.NoError(t, create(file))

	// case: file exists, will create .bkp file
	assert.NoError(t, create(file))

	// Remove created file. Fails if those files were not created.
	assert.NoError(t, os.Remove(file))
	assert.NoError(t, os.Remove(file+".bkp"))
	assert.NoError(t, os.Remove(filepath.Dir(file)))
}

func TestGetBoardSuggestionsFallsBackToNoneOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cases := []struct {
		name         string
		installation string
	}{
		{name: "cloud installation", installation: jira.InstallationTypeCloud},
		{name: "local installation", installation: jira.InstallationTypeLocal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gen := NewJiraCLIConfigGenerator(&JiraCLIConfig{})
			gen.value.installation = tc.installation
			gen.jiraClient = jira.NewClient(jira.Config{Server: server.URL}, jira.WithTimeout(3*time.Second))

			err := gen.getBoardSuggestions("TEST")
			assert.NoError(t, err)
			assert.Equal(t, []string{optionNone}, gen.boardSuggestions)
		})
	}
}

func TestConfigureMetadataUsesNewEndpointForCloud(t *testing.T) {
	var hitDeprecatedBulkEndpoint, hitNewEndpoint bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/issue/createmeta":
			// Atlassian has sunset this bulk endpoint for Jira Cloud. Cloud
			// installations must not hit it anymore.
			hitDeprecatedBulkEndpoint = true
			w.WriteHeader(http.StatusNotFound)
		case "/rest/api/2/issue/createmeta/TEST/issuetypes":
			hitNewEndpoint = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"values":[{"id":"10001","name":"Epic","subtask":false}]}`))
		case "/rest/api/2/field":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("unexpected request to %s", r.URL.Path)
		}
	}))
	defer server.Close()

	gen := &JiraCLIConfigGenerator{
		jiraClient: jira.NewClient(jira.Config{Server: server.URL}, jira.WithTimeout(3*time.Second)),
	}
	gen.value.installation = jira.InstallationTypeCloud
	gen.value.project = &projectConf{Key: "TEST"}

	err := gen.configureMetadata()
	assert.NoError(t, err)

	assert.False(t, hitDeprecatedBulkEndpoint)
	assert.True(t, hitNewEndpoint)

	assert.Len(t, gen.value.issueTypes, 1)
	assert.Equal(t, "10001", gen.value.issueTypes[0].ID)
	assert.Equal(t, "Epic", gen.value.issueTypes[0].Name)
}
