package edit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	adfDocFixture = `{"version":1,"type":"doc","content":[` +
		`{"type":"paragraph","attrs":{"localId":"abc"},"content":[{"type":"text","text":"new"}]}]}`
	mediaA = "d3a1a2f4-0000-4000-8000-00000000000a"
	mediaB = "d3a1a2f4-0000-4000-8000-00000000000b"
)

func mediaDoc(t *testing.T, ids ...string) *adf.ADF {
	t.Helper()

	nodes := make([]*adf.Node, 0, len(ids))
	for _, id := range ids {
		nodes = append(nodes, &adf.Node{
			NodeType: adf.NodeMediaSingle,
			Content: []*adf.Node{{
				NodeType:   adf.NodeMedia,
				Attributes: map[string]any{"id": id, "type": "file"},
			}},
		})
	}
	return &adf.ADF{Version: 1, DocType: "doc", Content: nodes}
}

func TestParseADFBody(t *testing.T) {
	const notADoc = `--adf-body must be an Atlassian document: a JSON object with "type": "doc", "version": 1 and a "content" array`

	valid := []struct {
		name, in string
	}{
		{"document", adfDocFixture},
		{"surrounding whitespace", "\n  " + adfDocFixture + "\n"},
		{"empty content clears the description", `{"version":1,"type":"doc","content":[]}`},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			raw, doc, err := parseADFBody([]byte(tc.in))
			require.NoError(t, err)
			require.NotNil(t, doc)
			assert.JSONEq(t, tc.in, string(raw))
		})
	}

	t.Run("byte order mark", func(t *testing.T) {
		raw, doc, err := parseADFBody([]byte("\xef\xbb\xbf" + adfDocFixture))
		require.NoError(t, err)
		require.NotNil(t, doc)
		assert.JSONEq(t, adfDocFixture, string(raw))
	})

	t.Run("sent unchanged, including what adf.Node doesn't model", func(t *testing.T) {
		raw, _, err := parseADFBody([]byte(adfDocFixture))
		require.NoError(t, err)
		assert.Contains(t, string(raw), `"localId":"abc"`)
	})

	invalid := []struct {
		name, in, wantErr string
	}{
		{"empty", "", "--adf-body is empty"},
		{"whitespace", " \n\t", "--adf-body is empty"},
		{"not json", "not json", "--adf-body is not valid JSON"},
		{
			"view --raw output",
			`{"key":"TEST-1","fields":{"description":` + adfDocFixture + `}}`,
			".fields.description",
		},
		{"wrong type", `{"version":1,"type":"paragraph","content":[]}`, notADoc},
		{"wrong version", `{"version":2,"type":"doc","content":[]}`, notADoc},
		{"missing version", `{"type":"doc","content":[]}`, notADoc},
		{"missing content", `{"version":1,"type":"doc"}`, notADoc},
		{"content is an object", `{"version":1,"type":"doc","content":{}}`, notADoc},
		{"content is null", `{"version":1,"type":"doc","content":null}`, notADoc},
		{"array", `[]`, notADoc},
		{"string", `"text"`, notADoc},
		{"null", `null`, notADoc},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			raw, doc, err := parseADFBody([]byte(tc.in))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Nil(t, raw)
			assert.Nil(t, doc)
		})
	}
}

func TestCheckADFImageLoss(t *testing.T) {
	external := &adf.ADF{Version: 1, DocType: "doc", Content: []*adf.Node{{
		NodeType:   adf.NodeMedia,
		Attributes: map[string]any{"type": "external", "url": "https://example.atlassian.net/image.png"},
	}}}

	tests := []struct {
		name          string
		current, next *adf.ADF
		allow         bool
		wantErr       string
	}{
		{name: "no current description", current: nil, next: mediaDoc(t)},
		{name: "media kept", current: mediaDoc(t, mediaA), next: mediaDoc(t, mediaA)},
		{name: "media kept among new ones", current: mediaDoc(t, mediaA), next: mediaDoc(t, mediaB, mediaA)},
		{
			name:    "media dropped",
			current: mediaDoc(t, mediaA), next: mediaDoc(t),
			wantErr: "the new description of TEST-1 drops 1 inline image(s) or attachment(s) of the current one; " +
				"keep their media nodes, or pass --allow-image-loss",
		},
		{
			name:    "one of two dropped",
			current: mediaDoc(t, mediaA, mediaB), next: mediaDoc(t, mediaB),
			wantErr: "drops 1 inline image(s)",
		},
		{
			name:    "both dropped",
			current: mediaDoc(t, mediaA, mediaB), next: mediaDoc(t),
			wantErr: "drops 2 inline image(s)",
		},
		{
			name:    "an id used twice counts once",
			current: mediaDoc(t, mediaA, mediaA), next: mediaDoc(t),
			wantErr: "drops 1 inline image(s)",
		},
		{name: "dropped but allowed", current: mediaDoc(t, mediaA), next: mediaDoc(t), allow: true},
		{name: "external media only", current: external, next: mediaDoc(t)},
		{name: "media added", current: mediaDoc(t), next: mediaDoc(t, mediaA)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkADFImageLoss("TEST-1", tc.current, tc.next, tc.allow)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestCheckADFBodyInstallation(t *testing.T) {
	assert.NoError(t, checkADFBodyInstallation(&editParams{}, true))
	assert.NoError(t, checkADFBodyInstallation(&editParams{}, false))
	assert.NoError(t, checkADFBodyInstallation(&editParams{adfBody: "doc.json"}, false))

	err := checkADFBodyInstallation(&editParams{adfBody: "doc.json"}, true)
	assert.EqualError(t, err, "--adf-body needs Jira Cloud; Jira Server and Data Center store the description "+
		"as wiki markup, use --body or --template")
}

func TestLoadADFBody(t *testing.T) {
	t.Run("not set", func(t *testing.T) {
		params := &editParams{}
		require.NoError(t, loadADFBody(params))
		assert.Nil(t, params.adfDoc)
	})

	t.Run("reads the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "doc.json")
		require.NoError(t, os.WriteFile(path, []byte(adfDocFixture), 0o600))

		params := &editParams{adfBody: path}
		require.NoError(t, loadADFBody(params))
		assert.JSONEq(t, adfDocFixture, string(params.adfDoc))
		assert.NotNil(t, params.adfParsed)
	})

	t.Run("missing file", func(t *testing.T) {
		params := &editParams{adfBody: filepath.Join(t.TempDir(), "missing.json")}
		err := loadADFBody(params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to read --adf-body")
	})

	t.Run("invalid content", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "doc.json")
		require.NoError(t, os.WriteFile(path, []byte("nope"), 0o600))

		err := loadADFBody(&editParams{adfBody: path})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not valid JSON")
	})
}

func TestAdfDescriptionWriteSendsTheDocumentOnly(t *testing.T) {
	var requests []string
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	client := jira.NewClient(jira.Config{Server: srv.URL})

	write := adfDescriptionWrite(client, "TEST-1", json.RawMessage(adfDocFixture))
	require.NoError(t, write(true))

	// The user's document is not merged with the current description, so
	// there is nothing to read first.
	assert.Equal(t, []string{"PUT /rest/api/3/issue/TEST-1?notifyUsers=false"}, requests)
	assert.JSONEq(t, `{"fields":{"description":`+adfDocFixture+`}}`, string(body))
}
