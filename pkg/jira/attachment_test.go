package jira

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetIssueAttachments(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/rest/api/2/issue/TEST-1", r.URL.Path)
		assert.Equal(t, "fields=attachment", r.URL.RawQuery)

		if unexpectedStatusCode {
			w.WriteHeader(400)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fields":{"attachment":[
			{"id":"10","filename":"a.txt","size":12,"created":"2026-01-01T10:00:00.000+0000","author":{"displayName":"Person A"},"content":"https://x/a"},
			{"id":"11","filename":"b.png","size":2048}
		]}}`))
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	out, err := client.GetIssueAttachments("TEST-1")
	assert.NoError(t, err)
	assert.Len(t, out, 2)
	assert.Equal(t, "10", out[0].ID)
	assert.Equal(t, "a.txt", out[0].Filename)
	assert.Equal(t, int64(12), out[0].Size)
	assert.Equal(t, "Person A", out[0].Author.DisplayName)

	unexpectedStatusCode = true

	_, err = client.GetIssueAttachments("TEST-1")
	assertUnexpectedResponse(t, err)
}

func TestAddIssueAttachment(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/rest/api/2/issue/TEST-1/attachments", r.URL.Path)
		assert.Equal(t, "no-check", r.Header.Get("X-Atlassian-Token"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.True(t, strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data; boundary="))

		file, header, err := r.FormFile("file")
		if !assert.NoError(t, err) {
			w.WriteHeader(400)
			return
		}
		defer func() { _ = file.Close() }()

		assert.Equal(t, "report.txt", header.Filename)
		content, _ := io.ReadAll(file)
		assert.Equal(t, "hello", string(content))

		if unexpectedStatusCode {
			w.WriteHeader(403)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"10","filename":"report.txt","size":5}]`))
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	out, err := client.AddIssueAttachment("TEST-1", "/some/dir/report.txt", strings.NewReader("hello"))
	assert.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "10", out[0].ID)
	assert.Equal(t, "report.txt", out[0].Filename)

	unexpectedStatusCode = true

	_, err = client.AddIssueAttachment("TEST-1", "report.txt", strings.NewReader("hello"))
	assertUnexpectedResponse(t, err)
}

func TestDownloadAttachment(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)

		if unexpectedStatusCode {
			w.WriteHeader(404)
			return
		}

		switch r.URL.Path {
		case "/rest/api/3/attachment/content/10":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("cloud content"))
		case "/jira/secure/attachment/11/a b.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("server content"))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	t.Run("cloud link is re-based onto the configured server", func(t *testing.T) {
		client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

		var buf bytes.Buffer
		err := client.DownloadAttachment(Attachment{
			Filename: "a.txt",
			Content:  "https://example.atlassian.net/rest/api/3/attachment/content/10",
		}, &buf)
		assert.NoError(t, err)
		assert.Equal(t, "cloud content", buf.String())
	})

	t.Run("server link is re-based onto the configured server", func(t *testing.T) {
		client := NewClient(Config{Server: server.URL + "/jira"}, WithTimeout(3*time.Second))

		var buf bytes.Buffer
		err := client.DownloadAttachment(Attachment{
			Filename: "a b.txt",
			Content:  "http://jira.internal.example.com/jira/secure/attachment/11/a%20b.txt",
		}, &buf)
		assert.NoError(t, err)
		assert.Equal(t, "server content", buf.String())
	})

	t.Run("unrecognized link is refused", func(t *testing.T) {
		client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

		err := client.DownloadAttachment(Attachment{
			Filename: "a.txt",
			Content:  "https://evil.example.com/files/a.txt",
		}, io.Discard)
		assert.ErrorContains(t, err, "unrecognized download link")
	})

	t.Run("missing link", func(t *testing.T) {
		client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

		err := client.DownloadAttachment(Attachment{Filename: "a.txt"}, io.Discard)
		assert.Error(t, err)
	})

	t.Run("unexpected status", func(t *testing.T) {
		unexpectedStatusCode = true
		defer func() { unexpectedStatusCode = false }()

		client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

		err := client.DownloadAttachment(Attachment{
			Filename: "a.txt",
			Content:  server.URL + "/rest/api/3/attachment/content/10",
		}, io.Discard)
		assertUnexpectedResponse(t, err)
	})
}

func TestDownloadAttachmentRedirectDropsCredentials(t *testing.T) {
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("media content"))
	}))
	defer media.Close()

	// Go keeps credentials on redirects within the same host name, so the
	// media server must be reached under a different one, as on Cloud.
	mediaURL := strings.Replace(media.URL, "127.0.0.1", "localhost", 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.Header.Get("Authorization"))
		http.Redirect(w, r, mediaURL+"/file/10/binary", http.StatusSeeOther)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL, Login: "user", APIToken: "secret"}, WithTimeout(3*time.Second))

	var buf bytes.Buffer
	err := client.DownloadAttachment(Attachment{
		Filename: "a.txt",
		Content:  "https://example.atlassian.net/rest/api/2/attachment/content/10",
	}, &buf)
	assert.NoError(t, err)
	assert.Equal(t, "media content", buf.String())
}
