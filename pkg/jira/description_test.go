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
	"github.com/stretchr/testify/require"

	"github.com/rethab/jira-cli/pkg/adf"
)

func serveDescription(t *testing.T, path, body string) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, path, r.URL.Path)
		assert.Equal(t, "description", r.URL.Query().Get("fields"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))
}

func TestGetIssueDescription(t *testing.T) {
	fixture, err := os.ReadFile("./testdata/issue-description-rich.json")
	require.NoError(t, err)

	t.Run("rich description survives a round trip untouched", func(t *testing.T) {
		client := serveDescription(t, "/rest/api/3/issue/TEST-1", string(fixture))

		doc, err := client.GetIssueDescription("TEST-1")
		require.NoError(t, err)
		require.NotNil(t, doc)

		var want struct {
			Fields struct {
				Description json.RawMessage `json:"description"`
			} `json:"fields"`
		}
		require.NoError(t, json.Unmarshal(fixture, &want))

		got, err := json.Marshal(doc)
		require.NoError(t, err)
		assert.JSONEq(t, string(want.Fields.Description), string(got))
	})

	t.Run("null description is nil", func(t *testing.T) {
		client := serveDescription(t, "/rest/api/3/issue/TEST-1", `{"fields":{"description":null}}`)

		doc, err := client.GetIssueDescription("TEST-1")
		assert.NoError(t, err)
		assert.Nil(t, doc)
	})

	t.Run("missing description is nil", func(t *testing.T) {
		client := serveDescription(t, "/rest/api/3/issue/TEST-1", `{"fields":{}}`)

		doc, err := client.GetIssueDescription("TEST-1")
		assert.NoError(t, err)
		assert.Nil(t, doc)
	})

	t.Run("a description that isn't ADF is an error and not an empty one", func(t *testing.T) {
		for name, body := range map[string]string{
			"string":      `{"fields":{"description":"h1. wiki"}}`,
			"wrong type":  `{"fields":{"description":{"version":1,"type":"paragraph","content":[]}}}`,
			"empty":       `{"fields":{"description":{}}}`,
			"array":       `{"fields":{"description":[]}}`,
			"bad content": `{"fields":{"description":{"version":1,"type":"doc","content":"x"}}}`,
		} {
			t.Run(name, func(t *testing.T) {
				client := serveDescription(t, "/rest/api/3/issue/TEST-1", body)

				doc, err := client.GetIssueDescription("TEST-1")
				assert.Error(t, err)
				assert.Nil(t, doc)
			})
		}
	})

	t.Run("unexpected status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist"]}`))
		}))
		defer server.Close()

		client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

		_, err := client.GetIssueDescription("TEST-1")
		var unexpected *ErrUnexpectedResponse
		assert.ErrorAs(t, err, &unexpected)
	})
}

func TestGetIssueDescriptionV2(t *testing.T) {
	t.Run("wiki markup", func(t *testing.T) {
		client := serveDescription(t, "/rest/api/2/issue/TEST-1", `{"fields":{"description":"h1. Title\n* a"}}`)

		got, err := client.GetIssueDescriptionV2("TEST-1")
		assert.NoError(t, err)
		assert.Equal(t, "h1. Title\n* a", got)
	})

	t.Run("null description is empty", func(t *testing.T) {
		client := serveDescription(t, "/rest/api/2/issue/TEST-1", `{"fields":{"description":null}}`)

		got, err := client.GetIssueDescriptionV2("TEST-1")
		assert.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a description that isn't a string is an error", func(t *testing.T) {
		client := serveDescription(t, "/rest/api/2/issue/TEST-1", `{"fields":{"description":{"type":"doc"}}}`)

		_, err := client.GetIssueDescriptionV2("TEST-1")
		assert.Error(t, err)
	})
}

type capturedWrite struct {
	method, path, query, body string
}

func captureWrite(t *testing.T, status int, response string) (*Client, *capturedWrite) {
	t.Helper()

	got := &capturedWrite{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		got.method, got.path, got.query, got.body = r.Method, r.URL.Path, r.URL.RawQuery, string(b)

		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)

	return NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second)), got
}

func TestSetIssueDescription(t *testing.T) {
	doc := &adf.ADF{
		Version: 1,
		DocType: "doc",
		Content: []*adf.Node{
			{NodeType: adf.NodeParagraph, Content: []*adf.Node{
				{NodeType: adf.ChildNodeText, NodeValue: adf.NodeValue{Text: "hi"}},
			}},
		},
	}
	want := `{"fields":{"description":{"version":1,"type":"doc","content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"hi"}]}]}}}`

	t.Run("sends only the description to v3", func(t *testing.T) {
		client, got := captureWrite(t, http.StatusNoContent, "")

		assert.NoError(t, client.SetIssueDescription("TEST-1", doc, false))
		assert.Equal(t, http.MethodPut, got.method)
		assert.Equal(t, "/rest/api/3/issue/TEST-1", got.path)
		assert.Empty(t, got.query)
		assert.JSONEq(t, want, got.body)
	})

	t.Run("skip notify", func(t *testing.T) {
		client, got := captureWrite(t, http.StatusNoContent, "")

		assert.NoError(t, client.SetIssueDescription("TEST-1", doc, true))
		assert.Equal(t, "notifyUsers=false", got.query)
	})

	t.Run("unexpected response", func(t *testing.T) {
		client, _ := captureWrite(t, http.StatusBadRequest, `{"errorMessages":["bad adf"]}`)

		err := client.SetIssueDescription("TEST-1", doc, false)
		var unexpected *ErrUnexpectedResponse
		require.ErrorAs(t, err, &unexpected)
		assert.Equal(t, http.StatusBadRequest, unexpected.StatusCode)
	})
}

func TestSetIssueDescriptionV2(t *testing.T) {
	t.Run("sends the wiki markup to v2", func(t *testing.T) {
		client, got := captureWrite(t, http.StatusNoContent, "")

		assert.NoError(t, client.SetIssueDescriptionV2("TEST-1", "h1. Hi\n* a", false))
		assert.Equal(t, http.MethodPut, got.method)
		assert.Equal(t, "/rest/api/2/issue/TEST-1", got.path)
		assert.Empty(t, got.query)
		assert.JSONEq(t, `{"fields":{"description":"h1. Hi\n* a"}}`, got.body)
	})

	t.Run("skip notify", func(t *testing.T) {
		client, got := captureWrite(t, http.StatusNoContent, "")

		assert.NoError(t, client.SetIssueDescriptionV2("TEST-1", "x", true))
		assert.Equal(t, "notifyUsers=false", got.query)
	})

	t.Run("unexpected response", func(t *testing.T) {
		client, _ := captureWrite(t, http.StatusConflict, `{"errorMessages":["conflict"]}`)

		err := client.SetIssueDescriptionV2("TEST-1", "x", false)
		var unexpected *ErrUnexpectedResponse
		assert.ErrorAs(t, err, &unexpected)
	})
}
