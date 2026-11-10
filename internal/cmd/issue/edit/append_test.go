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

func TestCheckDescriptionAdditions(t *testing.T) {
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
		{name: "prepend alone", params: &editParams{prependBody: "x", prependBodySet: true}},
		{
			name:    "prepend with body",
			params:  &editParams{prependBody: "x", prependBodySet: true, body: "b"},
			wantErr: "--prepend-body can't be combined with --body or --template",
		},
		{
			name:    "prepend with template",
			params:  &editParams{prependBody: "x", prependBodySet: true, template: "t.tmpl"},
			wantErr: "--prepend-body can't be combined with --body or --template",
		},
		{
			name:    "prepend whitespace only",
			params:  &editParams{prependBody: "  \n", prependBodySet: true},
			wantErr: "--prepend-body is empty",
		},
		{
			name:    "prepend explicitly empty",
			params:  &editParams{prependBody: "", prependBodySet: true},
			wantErr: "--prepend-body is empty",
		},
		{
			name: "prepend and append",
			params: &editParams{
				prependBody: "x", prependBodySet: true, appendBody: "y", appendBodySet: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkDescriptionAdditions(tc.params)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestQuestionsSkipDescriptionEditorWhenExtending(t *testing.T) {
	ec := editCmd{params: &editParams{summary: "kept", appendBody: "more"}}
	assert.Empty(t, ec.questions(&jira.Issue{}, ""))

	ec = editCmd{params: &editParams{appendBody: "more"}}
	qs := ec.questions(&jira.Issue{}, "")
	require.Len(t, qs, 1)
	assert.Equal(t, "summary", qs[0].Name)

	ec = editCmd{params: &editParams{summary: "kept", prependBody: "more"}}
	assert.Empty(t, ec.questions(&jira.Issue{}, ""))

	ec = editCmd{params: &editParams{prependBody: "more"}}
	qs = ec.questions(&jira.Issue{}, "")
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
		{name: "prepend body given", params: &editParams{prependBody: "p"}, stdin: true, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, shouldReadBodyFromStdin(tc.params, tc.stdin))
		})
	}
}

func TestOnlyAddingToDescription(t *testing.T) {
	tests := []struct {
		name   string
		params *editParams
		want   bool
	}{
		{name: "append alone", params: &editParams{appendBody: "a"}, want: true},
		{name: "append and assignee", params: &editParams{appendBody: "a", assignee: "me"}, want: true},
		{name: "append and notify flag", params: &editParams{appendBody: "a", skipNotify: true}, want: true},
		{name: "prepend alone", params: &editParams{prependBody: "p"}, want: true},
		{name: "prepend and append", params: &editParams{prependBody: "p", appendBody: "a"}, want: true},
		{name: "prepend and summary", params: &editParams{prependBody: "p", summary: "s"}, want: false},
		{name: "nothing to add", params: &editParams{}, want: false},
		{name: "summary", params: &editParams{appendBody: "a", summary: "s"}, want: false},
		{name: "issue type", params: &editParams{appendBody: "a", issueType: "Bug"}, want: false},
		{name: "parent", params: &editParams{appendBody: "a", parentIssueKey: "P-1"}, want: false},
		{name: "priority", params: &editParams{appendBody: "a", priority: "High"}, want: false},
		{name: "original estimate", params: &editParams{appendBody: "a", originalEstimate: "1d"}, want: false},
		{name: "labels", params: &editParams{appendBody: "a", labels: []string{"l"}}, want: false},
		{name: "components", params: &editParams{appendBody: "a", components: []string{"c"}}, want: false},
		{name: "fix versions", params: &editParams{appendBody: "a", fixVersions: []string{"v"}}, want: false},
		{name: "affects versions", params: &editParams{appendBody: "a", affectsVersions: []string{"v"}}, want: false},
		{name: "custom fields", params: &editParams{appendBody: "a", customFields: map[string]string{"k": "v"}}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, onlyAddingToDescription(tc.params))
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

func TestPrependWiki(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		added    string
		want     string
	}{
		{name: "empty existing", existing: "", added: "new", want: "new"},
		{name: "blank existing", existing: "\n\n", added: "new", want: "new"},
		{name: "nothing added", existing: "old", added: "", want: "old"},
		{name: "paragraphs", existing: "old", added: "new", want: "new\n\nold"},
		{name: "leading newlines of existing are dropped", existing: "\n\nold", added: "new", want: "new\n\nold"},
		{name: "trailing newlines of added are dropped", existing: "old", added: "new\n\n", want: "new\n\nold"},
		{name: "bullet list joins bullet list", existing: "* a\n* b\npara", added: "* c", want: "* c\n* a\n* b\npara"},
		{name: "dash list joins bullet list", existing: "* a", added: "- c", want: "- c\n* a"},
		{name: "numbered list joins numbered list", existing: "# a\n# b", added: "# c", want: "# c\n# a\n# b"},
		{name: "bullet list before numbered list stay apart", existing: "# a", added: "* b", want: "* b\n\n# a"},
		{name: "list before paragraph", existing: "text", added: "* a", want: "* a\n\ntext"},
		{name: "paragraph before list", existing: "* a", added: "text", want: "text\n\n* a"},
		{name: "bold line is not a list", existing: "*bold*", added: "* a", want: "* a\n\n*bold*"},
		{name: "list joins after other added content", existing: "* a", added: "text\n\n* z", want: "text\n\n* z\n* a"},
		{
			name:     "existing image line is kept",
			existing: "!diagram.png|thumbnail!\nold",
			added:    "new",
			want:     "new\n\n!diagram.png|thumbnail!\nold",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, prependWiki(tc.existing, tc.added))
		})
	}
}

type recordedWrite struct {
	method, path, query, body string
	puts                      int
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
			*written = recordedWrite{r.Method, r.URL.Path, r.URL.RawQuery, string(b), written.puts + 1}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	return srv, written
}

func TestExtendDescriptionAppendCloud(t *testing.T) {
	const existing = `{"version":1,"type":"doc","content":[` +
		`{"type":"panel","attrs":{"panelType":"info"},"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"Heads up"}]}]},` +
		`{"type":"bulletList","content":[{"type":"listItem","content":[` +
		`{"type":"paragraph","content":[` +
		`{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/browse/ABC-1"}},` +
		`{"type":"text","text":" first","marks":[{"type":"link","attrs":{"href":"https://x","title":"T"}}]}]}]}]}]}`

	srv, written := descriptionServer(t, "/rest/api/3/issue/TEST-1", existing)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeCloud})

	err := extendDescription(client, "TEST-1", "", "- second\n\nafter", false, true)
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

func TestExtendDescriptionAppendCloudEmptyDescription(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/3/issue/TEST-1", "null")
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeCloud})

	require.NoError(t, extendDescription(client, "TEST-1", "", "hello", false, false))

	assert.Empty(t, written.query)
	assert.JSONEq(t, `{"fields":{"description":{"version":1,"type":"doc","content":[`+
		`{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}}}`, written.body)
}

func TestExtendDescriptionAppendCloudKeepsDescriptionThatIsNotADF(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/3/issue/TEST-1", `"plain string"`)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeCloud})

	assert.Error(t, extendDescription(client, "TEST-1", "", "hello", false, false))
	assert.Empty(t, written.method, "nothing may be written")
}

func TestExtendDescriptionAppendServer(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/2/issue/TEST-1", `"h1. Title\n* a\n* b"`)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeLocal})

	require.NoError(t, extendDescription(client, "TEST-1", "", "- c\n\nafter", true, true))

	assert.Equal(t, http.MethodPut, written.method)
	assert.Equal(t, "/rest/api/2/issue/TEST-1", written.path)
	assert.Equal(t, "notifyUsers=false", written.query)
	assert.JSONEq(t, `{"fields":{"description":"h1. Title\n* a\n* b\n* c\n\nafter"}}`, written.body)
}

func TestExtendDescriptionAppendServerEmptyDescription(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/2/issue/TEST-1", "null")
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeLocal})

	require.NoError(t, extendDescription(client, "TEST-1", "", "hello", true, false))

	assert.JSONEq(t, `{"fields":{"description":"hello"}}`, written.body)
}

func TestExtendDescriptionCloudPrependKeepsMedia(t *testing.T) {
	const media = `{"type":"mediaSingle","attrs":{"layout":"center","width":50},"content":[` +
		`{"type":"media","attrs":{"id":"d3a1a2f4-0000-4000-8000-000000000001","type":"file","collection":"","width":800,"height":600}}]}`
	const old = `{"type":"paragraph","content":[{"type":"text","text":"old"}]}`

	srv, written := descriptionServer(t, "/rest/api/3/issue/TEST-1", `{"version":1,"type":"doc","content":[`+media+`,`+old+`]}`)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeCloud})

	require.NoError(t, extendDescription(client, "TEST-1", "## Status\n\nnew", "", false, false))

	assert.Equal(t, 1, written.puts)
	assert.Equal(t, http.MethodPut, written.method)
	assert.Equal(t, "/rest/api/3/issue/TEST-1", written.path)
	assert.Empty(t, written.query)
	assert.JSONEq(t, `{"fields":{"description":{"version":1,"type":"doc","content":[`+
		`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Status"}]},`+
		`{"type":"paragraph","content":[{"type":"text","text":"new"}]},`+
		media+`,`+old+`]}}}`, written.body)
}

func TestExtendDescriptionCloudPrependAndAppend(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/3/issue/TEST-1",
		`{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"existing"}]}]}`)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeCloud})

	require.NoError(t, extendDescription(client, "TEST-1", "top", "bottom", false, true))

	assert.Equal(t, 1, written.puts)
	assert.Equal(t, "notifyUsers=false", written.query)
	assert.JSONEq(t, `{"fields":{"description":{"version":1,"type":"doc","content":[`+
		`{"type":"paragraph","content":[{"type":"text","text":"top"}]},`+
		`{"type":"paragraph","content":[{"type":"text","text":"existing"}]},`+
		`{"type":"paragraph","content":[{"type":"text","text":"bottom"}]}]}}}`, written.body)
}

func TestExtendDescriptionCloudPrependEmptyDescription(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/3/issue/TEST-1", "null")
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeCloud})

	require.NoError(t, extendDescription(client, "TEST-1", "hello", "", false, false))

	assert.JSONEq(t, `{"fields":{"description":{"version":1,"type":"doc","content":[`+
		`{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}}}`, written.body)
}

func TestExtendDescriptionCloudPrependKeepsDescriptionThatIsNotADF(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/3/issue/TEST-1", `"plain string"`)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeCloud})

	assert.Error(t, extendDescription(client, "TEST-1", "hello", "", false, false))
	assert.Zero(t, written.puts, "nothing may be written")
}

func TestExtendDescriptionServerPrepend(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/2/issue/TEST-1", `"* a\n* b\n\nafter"`)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeLocal})

	require.NoError(t, extendDescription(client, "TEST-1", "- z", "", true, false))

	assert.Equal(t, http.MethodPut, written.method)
	assert.Equal(t, "/rest/api/2/issue/TEST-1", written.path)
	assert.JSONEq(t, `{"fields":{"description":"* z\n* a\n* b\n\nafter"}}`, written.body)
}

func TestExtendDescriptionServerPrependAndAppend(t *testing.T) {
	srv, written := descriptionServer(t, "/rest/api/2/issue/TEST-1", `"!diagram.png!\nmiddle"`)
	client := jira.NewClient(jira.Config{Server: srv.URL, Installation: jira.InstallationTypeLocal})

	require.NoError(t, extendDescription(client, "TEST-1", "top", "bottom", true, true))

	assert.Equal(t, 1, written.puts)
	assert.Equal(t, "notifyUsers=false", written.query)
	assert.JSONEq(t, `{"fields":{"description":"top\n\n!diagram.png!\nmiddle\n\nbottom"}}`, written.body)
}
