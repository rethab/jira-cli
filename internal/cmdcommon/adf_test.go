package cmdcommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	adfDocFixture = `{"version":1,"type":"doc","content":[` +
		`{"type":"paragraph","attrs":{"localId":"abc"},"content":[{"type":"text","text":"new"}]}]}`
	adfSource = ".fields.description of jira issue view --raw"
)

func TestParseADFDocument(t *testing.T) {
	const notADoc = `--adf-body must be an Atlassian document: a JSON object with "type": "doc", "version": 1 and a "content" array`

	valid := []struct {
		name, in string
	}{
		{"document", adfDocFixture},
		{"surrounding whitespace", "\n  " + adfDocFixture + "\n"},
		{"empty content clears the field", `{"version":1,"type":"doc","content":[]}`},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			raw, doc, err := ParseADFDocument([]byte(tc.in), "--adf-body", adfSource)
			require.NoError(t, err)
			require.NotNil(t, doc)
			assert.JSONEq(t, tc.in, string(raw))
		})
	}

	t.Run("byte order mark", func(t *testing.T) {
		raw, doc, err := ParseADFDocument([]byte("\xef\xbb\xbf"+adfDocFixture), "--adf-body", adfSource)
		require.NoError(t, err)
		require.NotNil(t, doc)
		assert.JSONEq(t, adfDocFixture, string(raw))
	})

	t.Run("sent unchanged, including what adf.Node doesn't model", func(t *testing.T) {
		raw, _, err := ParseADFDocument([]byte(adfDocFixture), "--adf-body", adfSource)
		require.NoError(t, err)
		assert.Contains(t, string(raw), `"localId":"abc"`)
	})

	invalid := []struct {
		name, in, wantErr string
	}{
		{"empty", "", "--adf-body is empty"},
		{"whitespace", " \n\t", "--adf-body is empty"},
		{"not json", "not json", "--adf-body is not valid JSON"},
		{"wrong type", `{"version":1,"type":"paragraph","content":[]}`, notADoc},
		{"wrong version", `{"version":2,"type":"doc","content":[]}`, notADoc},
		{"missing version", `{"type":"doc","content":[]}`, notADoc},
		{"missing content", `{"version":1,"type":"doc"}`, notADoc},
		{"content is an object", `{"version":1,"type":"doc","content":{}}`, notADoc},
		{"content is null", `{"version":1,"type":"doc","content":null}`, notADoc},
		{"string", `"text"`, notADoc},
		{"null", `null`, notADoc},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			raw, doc, err := ParseADFDocument([]byte(tc.in), "--adf-body", adfSource)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Nil(t, raw)
			assert.Nil(t, doc)
		})
	}

	t.Run("the flag is named in the message", func(t *testing.T) {
		_, _, err := ParseADFDocument([]byte(""), "--custom-adf customfield_10050", adfSource)
		assert.EqualError(t, err, "--custom-adf customfield_10050 is empty")
	})

	wrappers := []struct {
		name, in string
	}{
		{"whole issue", `{"key":"TEST-1","fields":{"description":` + adfDocFixture + `}}`},
		{"whole comment", `{"id":"10042","body":` + adfDocFixture + `}`},
		{"comment list", `[{"id":"10042","body":` + adfDocFixture + `}]`},
		{"empty comment list", `[]`},
	}
	for _, tc := range wrappers {
		t.Run("wrapper: "+tc.name, func(t *testing.T) {
			raw, doc, err := ParseADFDocument([]byte(tc.in), "--adf-body", ".body of one comment")
			require.Error(t, err)
			assert.Equal(t, "--adf-body takes the document itself, not what surrounds it; pass .body of one comment", err.Error())
			assert.Nil(t, raw)
			assert.Nil(t, doc)
		})
	}

	t.Run("a document with a body key is not mistaken for a comment", func(t *testing.T) {
		_, _, err := ParseADFDocument([]byte(`{"version":1,"type":"doc","body":1,"content":[]}`), "--adf-body", adfSource)
		assert.NoError(t, err)
	})
}
