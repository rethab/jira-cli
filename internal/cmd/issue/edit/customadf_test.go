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

	"github.com/rethab/jira-cli/pkg/jira"
)

func field(key, name string) jira.IssueTypeField {
	return jira.IssueTypeField{Key: key, Name: name}
}

func TestParseCustomADF(t *testing.T) {
	valid := []struct {
		name, in, key, path string
	}{
		{"by id", "customfield_10050=doc.json", "customfield_10050", "doc.json"},
		{"by name with a space", "Acceptance criteria=doc.json", "Acceptance criteria", "doc.json"},
		{"split on the first equals sign", "f=a=b.json", "f", "a=b.json"},
		{"standard input", "f=-", "f", "-"},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCustomADF([]string{tc.in})
			require.NoError(t, err)
			assert.Equal(t, []customADFValue{{key: tc.key, path: tc.path}}, got)
		})
	}

	for _, in := range []string{"nodelimiter", "=doc.json", "f=", ""} {
		t.Run("invalid "+in, func(t *testing.T) {
			_, err := parseCustomADF([]string{in})
			assert.EqualError(t, err, `invalid --custom-adf "`+in+`": want <field>=<path|->`)
		})
	}

	t.Run("keeps the order and repeats", func(t *testing.T) {
		got, err := parseCustomADF([]string{"a=1.json", "b=2.json"})
		require.NoError(t, err)
		assert.Equal(t, []customADFValue{{key: "a", path: "1.json"}, {key: "b", path: "2.json"}}, got)
	})
}

func TestCheckCustomADF(t *testing.T) {
	tests := []struct {
		name    string
		params  *editParams
		local   bool
		wantErr string
	}{
		{name: "not set", params: &editParams{}, local: true},
		{name: "cloud", params: &editParams{customADF: []customADFValue{{key: "f", path: "doc.json"}}}},
		{
			name: "same key twice",
			params: &editParams{customADF: []customADFValue{
				{key: "Acceptance criteria", path: "a.json"}, {key: "acceptance criteria", path: "b.json"},
			}},
			wantErr: `--custom-adf is given "acceptance criteria" more than once`,
		},
		{
			name: "key also in --custom",
			params: &editParams{
				customADF:    []customADFValue{{key: "Acceptance criteria", path: "a.json"}},
				customFields: map[string]string{"acceptance criteria": "text"},
			},
			wantErr: `"Acceptance criteria" is given to both --custom and --custom-adf`,
		},
		{
			name: "--adf-body - with --custom-adf -",
			params: &editParams{
				adfBody:   "-",
				customADF: []customADFValue{{key: "f", path: "-"}},
			},
			wantErr: "only one of --template, --adf-body and --custom-adf can read standard input",
		},
		{
			name: "two --custom-adf -",
			params: &editParams{customADF: []customADFValue{
				{key: "f", path: "-"}, {key: "g", path: "-"},
			}},
			wantErr: "only one of --template, --adf-body and --custom-adf can read standard input",
		},
		{
			name: "--template - with --custom-adf -",
			params: &editParams{
				template:  "-",
				customADF: []customADFValue{{key: "f", path: "-"}},
			},
			wantErr: "only one of --template, --adf-body and --custom-adf can read standard input",
		},
		{
			name: "--template - overridden by --body leaves standard input free",
			params: &editParams{
				body:      "text",
				template:  "-",
				customADF: []customADFValue{{key: "f", path: "-"}},
			},
		},
		{
			name: "one standard input and files",
			params: &editParams{
				adfBody:   "doc.json",
				customADF: []customADFValue{{key: "f", path: "-"}, {key: "g", path: "g.json"}},
			},
		},
		{
			name:   "refused on Jira Server and Data Center",
			params: &editParams{customADF: []customADFValue{{key: "f", path: "doc.json"}}},
			local:  true,
			wantErr: "--custom-adf needs Jira Cloud; on Jira Server and Data Center, --custom writes " +
				"wiki markup as is",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkCustomADF(tc.params, tc.local)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestLoadCustomADF(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	require.NoError(t, os.WriteFile(good, []byte(adfDocFixture), 0o600))
	bad := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(bad, []byte("nope"), 0o600))
	wrapped := filepath.Join(dir, "issue.json")
	require.NoError(t, os.WriteFile(wrapped, []byte(`{"fields":{"customfield_10050":`+adfDocFixture+`}}`), 0o600))

	t.Run("reads every file", func(t *testing.T) {
		params := &editParams{customADF: []customADFValue{{key: "f", path: good}, {key: "g", path: good}}}
		require.NoError(t, loadCustomADF(params))
		for _, v := range params.customADF {
			assert.JSONEq(t, adfDocFixture, string(v.doc))
		}
	})

	t.Run("missing file", func(t *testing.T) {
		params := &editParams{customADF: []customADFValue{{key: "f", path: filepath.Join(dir, "missing.json")}}}
		err := loadCustomADF(params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to read --custom-adf f")
	})

	t.Run("invalid content names the field", func(t *testing.T) {
		err := loadCustomADF(&editParams{customADF: []customADFValue{{key: "f", path: bad}}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--custom-adf f is not valid JSON")
	})

	t.Run("a whole issue is pointed at the field", func(t *testing.T) {
		err := loadCustomADF(&editParams{customADF: []customADFValue{{key: "f", path: wrapped}}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), ".fields.<field id> of jira issue view --raw")
	})
}

func TestResolveCustomADF(t *testing.T) {
	doc := json.RawMessage(adfDocFixture)
	fields := []jira.IssueTypeField{
		field("customfield_10050", "Acceptance criteria"),
		field("customfield_10051", "Release notes"),
		field("customfield_10052", "Duplicate"),
		field("customfield_10053", "Duplicate"),
	}

	t.Run("name and id", func(t *testing.T) {
		got, err := resolveCustomADF([]customADFValue{
			{key: "acceptance-criteria", doc: doc},
			{key: "customfield_10051", doc: doc},
		}, nil, fields)
		require.NoError(t, err)
		assert.Equal(t, map[string]json.RawMessage{"customfield_10050": doc, "customfield_10051": doc}, got)
	})

	t.Run("nothing to resolve", func(t *testing.T) {
		got, err := resolveCustomADF(nil, map[string]string{"x": "y"}, fields)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("the id and the name of one field are one field", func(t *testing.T) {
		_, err := resolveCustomADF([]customADFValue{
			{key: "acceptance-criteria", doc: doc},
			{key: "customfield_10050", doc: doc},
		}, nil, fields)
		assert.EqualError(t, err, `--custom-adf "customfield_10050" and "acceptance-criteria" are the same field`)
	})

	t.Run("ambiguous name", func(t *testing.T) {
		_, err := resolveCustomADF([]customADFValue{{key: "Duplicate", doc: doc}}, nil, fields)
		assert.EqualError(t, err, `--custom-adf "Duplicate" matches more than one field; pass the field id instead`)
	})

	t.Run("unknown field", func(t *testing.T) {
		_, err := resolveCustomADF([]customADFValue{{key: "Nope", doc: doc}}, nil, fields)
		assert.EqualError(t, err, `--custom-adf "Nope" matches no custom field`)
	})

	t.Run("--custom and --custom-adf naming the same field", func(t *testing.T) {
		_, err := resolveCustomADF(
			[]customADFValue{{key: "customfield_10050", doc: doc}},
			map[string]string{"acceptance-criteria": "text"},
			fields,
		)
		assert.EqualError(t, err, `"acceptance-criteria" of --custom and "customfield_10050" of --custom-adf are the same field`)
	})

	t.Run("--custom on another field", func(t *testing.T) {
		_, err := resolveCustomADF(
			[]customADFValue{{key: "customfield_10050", doc: doc}},
			map[string]string{"customfield_10051": "text"},
			fields,
		)
		assert.NoError(t, err)
	})
}

func TestADFFieldsWrite(t *testing.T) {
	const other = `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[` +
		`{"type":"status","attrs":{"text":"DONE","color":"green"}}]}]}`

	tests := []struct {
		name   string
		params *editParams
		want   string
	}{
		{
			// The user's document is not merged with the current description, so
			// there is nothing to read first.
			name:   "the description only",
			params: &editParams{issueKey: "TEST-1", adfDoc: json.RawMessage(adfDocFixture)},
			want:   `{"fields":{"description":` + adfDocFixture + `}}`,
		},
		{
			name: "the description and custom fields in one request",
			params: &editParams{
				issueKey: "TEST-1",
				adfDoc:   json.RawMessage(adfDocFixture),
				customADFFields: map[string]json.RawMessage{
					"customfield_10050": json.RawMessage(adfDocFixture),
					"customfield_10051": json.RawMessage(other),
				},
			},
			want: `{"fields":{"description":` + adfDocFixture +
				`,"customfield_10050":` + adfDocFixture + `,"customfield_10051":` + other + `}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				requests []string
				body     []byte
			)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.RequestURI())
				body, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(srv.Close)
			client := jira.NewClient(jira.Config{Server: srv.URL})

			write := adfFieldsWrite(client, tc.params)
			require.NotNil(t, write)
			require.NoError(t, write(true))

			assert.Equal(t, []string{"PUT /rest/api/3/issue/TEST-1?notifyUsers=false"}, requests)
			assert.JSONEq(t, tc.want, string(body))
		})
	}
}

func TestADFFieldsWriteWithNothingToWrite(t *testing.T) {
	assert.Nil(t, adfFieldsWrite(nil, &editParams{issueKey: "TEST-1"}))
}
