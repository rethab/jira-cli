package jira

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	smartLinkServer = "https://example.atlassian.net"
	smartLinkMD     = "See https://example.atlassian.net/browse/ABC-264): done"
	smartLinkCloud  = `See [https://example.atlassian.net/browse/ABC-264|https://example.atlassian.net/browse/ABC-264|smart-link]\): done`
	smartLinkLocal  = `See [https://example.atlassian.net/browse/ABC\-264|https://example.atlassian.net/browse/ABC-264]\): done`
)

func TestClientToJiraMDSmartLinks(t *testing.T) {
	tests := []struct {
		name         string
		installation string
		expected     string
	}{
		{name: "cloud", installation: InstallationTypeCloud, expected: smartLinkCloud},
		{name: "server", installation: InstallationTypeLocal, expected: smartLinkLocal},
		{name: "cloud in lower case", installation: "cloud", expected: smartLinkCloud},
		{name: "unknown installation", installation: "", expected: smartLinkLocal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(Config{Server: smartLinkServer + "/", Installation: tc.installation})

			assert.Equal(t, tc.expected, client.ToJiraMD(smartLinkMD))
		})
	}
}

// With a scoped token, the server is the API gateway, which nobody links to.
func TestClientToJiraMDSmartLinksUseBrowseServer(t *testing.T) {
	client := NewClient(Config{
		Server:       "https://api.atlassian.com/ex/jira/cloud-id",
		BrowseServer: smartLinkServer,
		Installation: InstallationTypeCloud,
	})

	assert.Equal(t, smartLinkCloud, client.ToJiraMD(smartLinkMD))
}

func TestCreateRequestDescriptionSmartLinks(t *testing.T) {
	cloud := NewClient(Config{Server: smartLinkServer, Installation: InstallationTypeCloud})
	local := NewClient(Config{Server: smartLinkServer, Installation: InstallationTypeLocal})

	assert.Equal(t, smartLinkCloud, cloud.getRequestData(&CreateRequest{Body: smartLinkMD}).Fields.M.Description)
	assert.Equal(t, smartLinkLocal, local.getRequestData(&CreateRequest{Body: smartLinkMD}).Fields.M.Description)
}

func TestAddIssueCommentSmartLinks(t *testing.T) {
	var body string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"10"}`))
	}))
	defer server.Close()

	// The server in the config is what decides which URLs are issues of this site.
	client := NewClient(Config{Server: server.URL, Installation: InstallationTypeCloud}, WithTimeout(3*time.Second))

	_, err := client.AddIssueComment("TEST-1", "Blocked by "+server.URL+"/browse/ABC-1.", false)
	require.NoError(t, err)
	assert.Contains(t, body, `Blocked by [`+server.URL+`/browse/ABC-1|`+server.URL+`/browse/ABC-1|smart-link].`)
}

func TestEditIssueCommentSmartLinks(t *testing.T) {
	var body string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"10","body":"old"}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL, Installation: InstallationTypeCloud}, WithTimeout(3*time.Second))

	require.NoError(t, client.EditIssueComment("TEST-1", "10", "Blocked by "+server.URL+"/browse/ABC-1."))
	assert.Contains(t, body, `Blocked by [`+server.URL+`/browse/ABC-1|`+server.URL+`/browse/ABC-1|smart-link].`)
}
