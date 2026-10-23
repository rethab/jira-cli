package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rethab/jira-cli/pkg/adf"
)

// smartLinkCommentADF is a Cloud comment with a smart link and a hyphenated
// word, which the v2 api renders as escaped wiki markup.
const smartLinkCommentADF = `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[` +
	`{"type":"text","text":"jira-cli smart link test, see "},` +
	`{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/browse/KEY-1"}},` +
	`{"type":"text","text":"): done"}]}]}`

func TestGetIssueCommentsV3(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/3/issue/TEST-1/comment":
			assert.Equal(t, "startAt=0&maxResults=100", r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"total":1,"comments":[{"id":"10","body":` + smartLinkCommentADF + `}]}`))
		case "/rest/api/3/issue/TEST-1/comment/10":
			assert.Equal(t, "expand=properties", r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"id":"10","body":` + smartLinkCommentADF + `}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	assertSmartLinkBody := func(body any) {
		doc, ok := body.(*adf.ADF)
		require.True(t, ok, "body is ADF")
		para := doc.Content[0]
		assert.Equal(t, "jira-cli smart link test, see ", para.Content[0].Text)
		assert.Equal(t, adf.InlineNodeCard, para.Content[1].NodeType)
	}

	comments, err := client.GetIssueComments("TEST-1")
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assertSmartLinkBody(comments[0].Body)

	comment, err := client.GetIssueComment("TEST-1", "10")
	require.NoError(t, err)
	assertSmartLinkBody(comment.Body)
}

func TestGetIssueCommentV2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Equal(t, "/rest/api/2/issue/TEST-1/comment/10", r.URL.Path)
		assert.Equal(t, "expand=properties", r.URL.RawQuery)
		_, _ = w.Write([]byte(`{"id":"10","body":"some *bold*"}`))
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	comment, err := client.GetIssueCommentV2("TEST-1", "10")
	assert.NoError(t, err)
	assert.Equal(t, "some *bold*", comment.Body)
}

func TestGetIssueCommentsV2(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/rest/api/2/issue/TEST-1/comment", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		if unexpectedStatusCode {
			w.WriteHeader(400)
			return
		}

		page := commentPage{Total: 3}
		switch r.URL.RawQuery {
		case "startAt=0&maxResults=100":
			page.Comments = []Comment{{ID: "1", Body: "first"}, {ID: "2", Body: "second"}}
		case "startAt=2&maxResults=100":
			page.Comments = []Comment{{ID: "3", Body: "third", Author: User{DisplayName: "Person A"}}}
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		assert.NoError(t, json.NewEncoder(w).Encode(page))
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	out, err := client.GetIssueCommentsV2("TEST-1")
	assert.NoError(t, err)
	assert.Len(t, out, 3)
	assert.Equal(t, "3", out[2].ID)
	assert.Equal(t, "Person A", out[2].Author.DisplayName)

	unexpectedStatusCode = true

	_, err = client.GetIssueCommentsV2("TEST-1")
	assertUnexpectedResponse(t, err)
}

func TestGetIssueCommentsStopsOnEmptyPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":5,"comments":[]}`))
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	out, err := client.GetIssueComments("TEST-1")
	assert.NoError(t, err)
	assert.Empty(t, out)
}

func TestEditIssueComment(t *testing.T) {
	tests := []struct {
		name         string
		existing     string
		expectedBody string
	}{
		{
			name:         "public comment",
			existing:     `{"id":"10","body":"old"}`,
			expectedBody: `{"body":"*new*"}`,
		},
		{
			name:         "internal comment stays internal",
			existing:     `{"id":"10","body":"old","properties":[{"key":"other","value":1},{"key":"sd.public.comment","value":{"internal":true}}]}`,
			expectedBody: `{"body":"*new*","properties":[{"key":"sd.public.comment","value":{"internal":true}}]}`,
		},
		{
			name:         "restricted comment stays restricted",
			existing:     `{"id":"10","body":"old","visibility":{"type":"role","value":"Developers"}}`,
			expectedBody: `{"body":"*new*","visibility":{"type":"role","value":"Developers"}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var unexpectedStatusCode bool

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/rest/api/2/issue/TEST-1/comment/10", r.URL.Path)

				if r.Method == "GET" {
					assert.Equal(t, "expand=properties", r.URL.RawQuery)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(tc.existing))
					return
				}

				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				actual, _ := io.ReadAll(r.Body)
				assert.JSONEq(t, tc.expectedBody, string(actual))

				if unexpectedStatusCode {
					w.WriteHeader(400)
				} else {
					w.WriteHeader(200)
				}
			}))
			defer server.Close()

			client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

			err := client.EditIssueComment("TEST-1", "10", "**new**")
			assert.NoError(t, err)

			unexpectedStatusCode = true

			err = client.EditIssueComment("TEST-1", "10", "**new**")
			assertUnexpectedResponse(t, err)
		})
	}
}

func TestEditIssueCommentNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	err := client.EditIssueComment("TEST-1", "10", "new")
	assertUnexpectedResponse(t, err)
}

func TestDeleteIssueComment(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/rest/api/2/issue/TEST-1/comment/10", r.URL.Path)

		if unexpectedStatusCode {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	err := client.DeleteIssueComment("TEST-1", "10")
	assert.NoError(t, err)

	unexpectedStatusCode = true

	err = client.DeleteIssueComment("TEST-1", "10")
	assertUnexpectedResponse(t, err)
}
