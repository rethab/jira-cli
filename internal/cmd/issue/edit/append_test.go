package edit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rethab/jira-cli/pkg/jira"
)

func TestCheckAppendBody(t *testing.T) {
	tests := []struct {
		name    string
		params  *editParams
		wantErr string
	}{
		{name: "not set", params: &editParams{body: "b"}},
		{name: "alone", params: &editParams{appendBody: "x", appendBodySet: true}},
		{
			name:    "with body",
			params:  &editParams{appendBody: "x", appendBodySet: true, body: "b"},
			wantErr: "--append-body can't be combined with --body or --template",
		},
		{
			name:    "with template",
			params:  &editParams{appendBody: "x", appendBodySet: true, template: "t.tmpl"},
			wantErr: "--append-body can't be combined with --body or --template",
		},
		{
			name:    "whitespace only",
			params:  &editParams{appendBody: "  \n", appendBodySet: true},
			wantErr: "--append-body is empty",
		},
		{
			name:    "explicitly empty",
			params:  &editParams{appendBody: "", appendBodySet: true},
			wantErr: "--append-body is empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkAppendBody(tc.params)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestQuestionsSkipDescriptionEditorWhenAppending(t *testing.T) {
	ec := editCmd{params: &editParams{summary: "kept", appendBody: "more"}}
	assert.Empty(t, ec.questions(&jira.Issue{}, ""))

	ec = editCmd{params: &editParams{appendBody: "more"}}
	qs := ec.questions(&jira.Issue{}, "")
	require.Len(t, qs, 1)
	assert.Equal(t, "summary", qs[0].Name)
}

// Agents often run with a non-TTY stdin, and reading it would replace the
// description that is meant to be appended to.
func TestShouldReadBodyFromStdin(t *testing.T) {
	tests := []struct {
		name   string
		params *editParams
		stdin  bool
		want   bool
	}{
		{name: "nothing given and stdin has data", params: &editParams{}, stdin: true, want: true},
		{name: "stdin is a terminal", params: &editParams{}, stdin: false, want: false},
		{name: "body given", params: &editParams{body: "b"}, stdin: true, want: false},
		{name: "template given", params: &editParams{template: "t"}, stdin: true, want: false},
		{name: "append body given", params: &editParams{appendBody: "a"}, stdin: true, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, shouldReadBodyFromStdin(tc.params, tc.stdin))
		})
	}
}

func TestAppendWiki(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		added    string
		want     string
	}{
		{name: "empty existing", existing: "", added: "new", want: "new"},
		{name: "blank existing", existing: "\n\n", added: "new", want: "new"},
		{name: "paragraphs", existing: "old", added: "new", want: "old\n\nnew"},
		{name: "trailing newlines are dropped", existing: "old\n\n", added: "new", want: "old\n\nnew"},
		{name: "bullet list joins bullet list", existing: "para\n* a\n* b", added: "* c", want: "para\n* a\n* b\n* c"},
		{name: "dash list joins bullet list", existing: "- a", added: "* c", want: "- a\n* c"},
		{name: "nested bullets join", existing: "* a\n** b", added: "* c", want: "* a\n** b\n* c"},
		{name: "numbered list joins numbered list", existing: "# a\n# b", added: "# c", want: "# a\n# b\n# c"},
		{name: "bullet list then numbered list stay apart", existing: "* a", added: "# b", want: "* a\n\n# b"},
		{name: "list then paragraph", existing: "* a", added: "text", want: "* a\n\ntext"},
		{name: "paragraph then list", existing: "text", added: "* a", want: "text\n\n* a"},
		{name: "bold line is not a list", existing: "*bold*", added: "* a", want: "*bold*\n\n* a"},
		{name: "list joins and the rest follows", existing: "* a", added: "* b\n\ntext", want: "* a\n* b\n\ntext"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, appendWiki(tc.existing, tc.added))
		})
	}
}

type recordedWrite struct {
	method, path, query, body string
}

func descriptionServer(t *testing.T, getPath, description string) (*httptest.Server, *recordedWrite) {
	t.Helper()

	var mu sync.Mutex
	written := &recordedWrite{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch r.Method {
		case http.MethodGet:
			assert.Equal(t, getPath, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"fields":{"description":` + description + `}}`))
		case http.MethodPut:
			b, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			*written = recordedWrite{r.Method, r.URL.Path, r.URL.RawQuery, string(b)}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	return srv, written
}

func TestAppendDescriptionCloud(t *testing.T) {
	const existing = `{"version":1,"type":"doc","content":[` +
		`{"type":"panel","attrs":{"panelType":"info"},"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"Heads up"}]}]},` +
		`{"type":"bulletList","content":[{"type":"listItem","content":[` +
		`{"type":"paragraph","content":[` +
		`{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/browse/ABC-1"}},` +
		`{"type":"text","text":" first","marks":[{"type":"link","attrs":{"href":"https://x","title":"T"}}]}]}]}]}]}`

	srv, written := descriptionServer(t, "/rest/api/3/issue/TEST-1", existing)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeCloud})

	err := appendDescription(client, "TEST-1", "- second\n\nafter", false, true)
	require.NoError(t, err)

	assert.Equal(t, http.MethodPut, written.method)
	assert.Equal(t, "/rest/api/3/issue/TEST-1", written.path)
	assert.Equal(t, "notifyUsers=false", written.query)
	assert.JSONEq(t, `{"fields":{"description":{"version":1,"type":"doc","content":[`+
		`{"type":"panel","attrs":{"panelType":"info"},"content":[`+
		`{"type":"paragraph","content":[{"type":"text","text":"Heads up"}]}]},`+
		`{"type":"bulletList","content":[`+
		`{"type":"listItem","content":[{"type":"paragraph","content":[`+
		`{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/browse/ABC-1"}},`+
		`{"type":"text","text":" first","marks":[{"type":"link","attrs":{"href":"https://x","title":"T"}}]}]}]},`+
		`{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"second"}]}]}]},`+
		`{"type":"paragraph","content":[{"type":"text","text":"after"}]}]}}}`, written.body)
}

func TestAppendDescriptionCloudEmptyDescription(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/3/issue/TEST-1", "null")
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeCloud})

	require.NoError(t, appendDescription(client, "TEST-1", "hello", false, false))

	assert.Empty(t, written.query)
	assert.JSONEq(t, `{"fields":{"description":{"version":1,"type":"doc","content":[`+
		`{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}}}`, written.body)
}

func TestAppendDescriptionCloudKeepsDescriptionThatIsNotADF(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/3/issue/TEST-1", `"plain string"`)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeCloud})

	assert.Error(t, appendDescription(client, "TEST-1", "hello", false, false))
	assert.Empty(t, written.method, "nothing may be written")
}

func TestAppendDescriptionServer(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/2/issue/TEST-1", `"h1. Title\n* a\n* b"`)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeLocal})

	require.NoError(t, appendDescription(client, "TEST-1", "- c\n\nafter", true, true))

	assert.Equal(t, http.MethodPut, written.method)
	assert.Equal(t, "/rest/api/2/issue/TEST-1", written.path)
	assert.Equal(t, "notifyUsers=false", written.query)
	assert.JSONEq(t, `{"fields":{"description":"h1. Title\n* a\n* b\n* c\n\nafter"}}`, written.body)
}

func TestAppendDescriptionServerEmptyDescription(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/2/issue/TEST-1", "null")
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeLocal})

	require.NoError(t, appendDescription(client, "TEST-1", "hello", true, false))

	assert.JSONEq(t, `{"fields":{"description":"hello"}}`, written.body)
}
