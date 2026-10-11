package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
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

func TestUseScopedTokenServer(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/_edge/tenant_info", r.URL.Path)
		_, _ = w.Write([]byte(`{"cloudId":"abc-123"}`))
	}))
	defer site.Close()

	gen := NewJiraCLIConfigGenerator(&JiraCLIConfig{ScopedToken: true})
	gen.value.server = site.URL + "/"

	assert.NoError(t, gen.useScopedTokenServer(site.Client()))
	assert.Equal(t, "https://api.atlassian.com/ex/jira/abc-123", gen.value.server)
	assert.Equal(t, site.URL, gen.value.browseServer)
}

func TestUseScopedTokenServerReturnsLookupError(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer site.Close()

	gen := NewJiraCLIConfigGenerator(&JiraCLIConfig{ScopedToken: true})
	gen.value.server = site.URL

	err := gen.useScopedTokenServer(site.Client())
	assert.ErrorContains(t, err, "cloud ID")
	assert.Equal(t, site.URL, gen.value.server)
	assert.Empty(t, gen.value.browseServer)
}

func TestWriteBrowseServer(t *testing.T) {
	cases := []struct {
		name         string
		browseServer string
	}{
		{name: "scoped token", browseServer: "https://acme.atlassian.net"},
		{name: "classic token", browseServer: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")

			assert.NoError(t, create(path))

			gen := NewJiraCLIConfigGenerator(&JiraCLIConfig{})
			gen.value.server = "https://example.test"
			gen.value.browseServer = tc.browseServer

			_, err := gen.write(path)
			assert.NoError(t, err)

			got := viper.New()
			got.SetConfigFile(path)
			assert.NoError(t, got.ReadInConfig())
			assert.Equal(t, "https://example.test", got.GetString("server"))
			assert.Equal(t, tc.browseServer, got.GetString("browse_server"))
			assert.Equal(t, tc.browseServer != "", got.IsSet("browse_server"))
		})
	}
}

func TestConfigureInstallationTypeRejectsScopedTokenForLocal(t *testing.T) {
	gen := NewJiraCLIConfigGenerator(&JiraCLIConfig{Installation: "local", ScopedToken: true})

	assert.ErrorIs(t, gen.configureInstallationType(), ErrScopedTokenLocal)
}
