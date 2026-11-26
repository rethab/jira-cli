package edit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const adfDocFixture = `{"version":1,"type":"doc","content":[` +
	`{"type":"paragraph","attrs":{"localId":"abc"},"content":[` +
	`{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/browse/TEST-2"}}]}]}`

func TestCheckADFBody(t *testing.T) {
	tests := []struct {
		name    string
		in      adfBodyFlags
		local   bool
		wantErr string
	}{
		{name: "cloud", in: adfBodyFlags{path: "doc.json"}},
		{name: "stdin", in: adfBodyFlags{path: "-"}},
		{
			name:    "set but empty",
			in:      adfBodyFlags{},
			wantErr: "--adf-body needs a path, or - for standard input",
		},
		{
			name:    "with a body",
			in:      adfBodyFlags{path: "doc.json", body: "text"},
			wantErr: "--adf-body can't be combined with a body, --body or --template",
		},
		{
			name:    "with --template",
			in:      adfBodyFlags{path: "doc.json", template: "t.tmpl"},
			wantErr: "--adf-body can't be combined with a body, --body or --template",
		},
		{
			name:  "refused on Jira Server and Data Center",
			in:    adfBodyFlags{path: "doc.json"},
			local: true,
			wantErr: "--adf-body needs Jira Cloud; Jira Server and Data Center store comments as wiki markup, " +
				"use --body or --template",
		},
		{
			name:    "empty wins over the installation check",
			in:      adfBodyFlags{},
			local:   true,
			wantErr: "--adf-body needs a path, or - for standard input",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkADFBody(tc.in, tc.local)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestLoadADFBody(t *testing.T) {
	t.Run("reads the file and returns it unchanged", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "doc.json")
		require.NoError(t, os.WriteFile(path, []byte(adfDocFixture), 0o600))

		doc, err := loadADFBody(path)
		require.NoError(t, err)
		assert.JSONEq(t, adfDocFixture, string(doc))
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := loadADFBody(filepath.Join(t.TempDir(), "missing.json"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to read --adf-body")
	})

	t.Run("invalid content", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "doc.json")
		require.NoError(t, os.WriteFile(path, []byte("nope"), 0o600))

		_, err := loadADFBody(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not valid JSON")
	})

	t.Run("a whole comment is pointed at its body", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "comment.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"id":"10042","body":`+adfDocFixture+`}`), 0o600))

		_, err := loadADFBody(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), ".body of one comment of jira issue comment list --raw")
	})
}
