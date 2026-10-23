package edit

import (
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
)

func TestBuildEditRequestConvertsMarkdownBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantBody string
	}{
		{
			name:     "markdown body is converted to wiki markup",
			body:     "# Heading\n\nSome **bold** text",
			wantBody: "h1. Heading\nSome *bold* text",
		},
		{
			name:     "empty body is left untouched",
			body:     "",
			wantBody: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			edr := buildEditRequest(newClient(jira.InstallationTypeLocal), "TEST", &jira.Issue{}, &editParams{body: tc.body})

			assert.Equal(t, tc.wantBody, edr.Body)
		})
	}
}

func TestEditorBodyIsMarkdown(t *testing.T) {
	tests := []struct {
		name        string
		description interface{} // string on-prem/Server, *adf.ADF on Cloud
		want        string
	}{
		{
			name:        "on-prem/server wiki markup is translated to markdown",
			description: "h1. Heading\n\nSome *bold* text",
			want:        "# Heading\nSome **bold** text\n",
		},
		{
			name: "cloud ADF is translated to markdown",
			description: &adf.ADF{
				Version: 1,
				DocType: "doc",
				Content: []*adf.Node{
					{
						NodeType:   adf.NodeHeading,
						Attributes: map[string]any{"level": float64(1)},
						Content: []*adf.Node{
							{NodeType: adf.ChildNodeText, NodeValue: adf.NodeValue{Text: "Heading"}},
						},
					},
				},
			},
			want: "# Heading\n",
		},
		{
			name:        "missing description yields an empty body",
			description: nil,
			want:        "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issue := &jira.Issue{Fields: jira.IssueFields{Description: tc.description}}

			assert.Equal(t, tc.want, editorBody(issue))
		})
	}
}

// An on-prem/Server description is served as wiki markup but the editor always shows
// markdown, so an interactive edit must not re-interpret the wiki markup as markdown.
func TestServerInteractiveEditRoundTrip(t *testing.T) {
	const wiki = "h2. Heading\n\n# first ordered item\n# second ordered item\n\nSome *bold wiki* text\n"

	issue := &jira.Issue{Fields: jira.IssueFields{Description: wiki}}

	prefilled := editorBody(issue)
	assert.Equal(t, "## Heading\n\n- first ordered item\n- second ordered item\nSome **bold wiki** text\n", prefilled)

	// The user edits a single word in $EDITOR and saves.
	edited := strings.Replace(prefilled, "Heading", "Updated heading", 1)

	edr := buildEditRequest(newClient(jira.InstallationTypeLocal), "TEST", issue, &editParams{body: edited})

	assert.Equal(
		t,
		"h2. Updated heading\n* first ordered item\n* second ordered item\nSome *bold wiki* text",
		edr.Body,
	)
	assert.NotContains(t, edr.Body, "h1.", "wiki ordered list items must not become headings")
	assert.NotContains(t, edr.Body, "_bold wiki_", "wiki bold must not become italic")
}

func TestBuildEditRequestMergesIssueFieldsWithParams(t *testing.T) {
	issue := &jira.Issue{
		Fields: jira.IssueFields{
			Labels: []string{"existing"},
			Components: []struct {
				Name string `json:"name"`
			}{{Name: "BE"}},
			FixVersions: []struct {
				Name string `json:"name"`
			}{{Name: "v1.0"}},
			AffectsVersions: []struct {
				Name string `json:"name"`
			}{{Name: "v0.9"}},
			Parent: &struct {
				Key string `json:"key"`
			}{Key: "TEST-1"},
		},
	}

	t.Run("params are appended to the existing issue fields", func(t *testing.T) {
		params := &editParams{
			summary:         "New summary",
			priority:        "High",
			labels:          []string{"added"},
			components:      []string{"FE"},
			fixVersions:     []string{"v2.0"},
			affectsVersions: []string{"v1.9"},
			customFields:    map[string]string{"story-points": "5"},
			skipNotify:      true,
		}

		edr := buildEditRequest(newClient(jira.InstallationTypeLocal), "TEST", issue, params)

		assert.Equal(t, "New summary", edr.Summary)
		assert.Equal(t, "High", edr.Priority)
		assert.Equal(t, []string{"added", "existing"}, edr.Labels)
		assert.Equal(t, []string{"BE", "FE"}, edr.Components)
		assert.Equal(t, []string{"v1.0", "v2.0"}, edr.FixVersions)
		assert.Equal(t, []string{"v0.9", "v1.9"}, edr.AffectsVersions)
		assert.Equal(t, map[string]string{"story-points": "5"}, edr.CustomFields)
		assert.True(t, edr.SkipNotify)
	})

	t.Run("original estimate is passed on", func(t *testing.T) {
		edr := buildEditRequest(newClient(jira.InstallationTypeLocal), "TEST", issue, &editParams{originalEstimate: "2h"})

		assert.Equal(t, "2h", edr.OriginalEstimate)
	})

	t.Run("existing parent is kept when no parent is given", func(t *testing.T) {
		edr := buildEditRequest(newClient(jira.InstallationTypeLocal), "TEST", issue, &editParams{})

		assert.Equal(t, "TEST-1", edr.ParentIssueKey)
	})

	t.Run("given parent overrides the existing one and is qualified with the project", func(t *testing.T) {
		edr := buildEditRequest(newClient(jira.InstallationTypeLocal), "TEST", issue, &editParams{parentIssueKey: "2"})

		assert.Equal(t, "TEST-2", edr.ParentIssueKey)
	})
}

func TestCheckImageLoss(t *testing.T) {
	withMedia := &adf.ADF{
		Version: 1,
		DocType: "doc",
		Content: []*adf.Node{
			{NodeType: adf.NodeMediaSingle, Content: []*adf.Node{{NodeType: adf.NodeMedia}}},
		},
	}
	withoutMedia := &adf.ADF{
		Version: 1,
		DocType: "doc",
		Content: []*adf.Node{
			{NodeType: adf.NodeParagraph, Content: []*adf.Node{{NodeType: adf.ChildNodeText, NodeValue: adf.NodeValue{Text: "hi"}}}},
		},
	}

	tests := []struct {
		name        string
		description interface{}
		params      *editParams
		wantErr     bool
	}{
		{name: "body replaced, description has media", description: withMedia, params: &editParams{body: "new"}, wantErr: true},
		{name: "override flag allows replacement", description: withMedia, params: &editParams{body: "new", allowImageLoss: true}},
		{name: "description not touched", description: withMedia, params: &editParams{summary: "new summary"}},
		{name: "body replaced, no media", description: withoutMedia, params: &editParams{body: "new"}},
		{name: "server wiki description", description: "!image.png!", params: &editParams{body: "new"}},
		{name: "no description", description: nil, params: &editParams{body: "new"}},
		{name: "empty cloud description", description: (*adf.ADF)(nil), params: &editParams{body: "new"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issue := &jira.Issue{}
			issue.Fields.Description = tc.description

			err := checkImageLoss(issue, tc.params)

			if tc.wantErr {
				assert.ErrorContains(t, err, "--allow-image-loss")
				return
			}
			assert.NoError(t, err)
		})
	}
}

func newClient(installation string) *jira.Client {
	return jira.NewClient(jira.Config{Server: "https://example.atlassian.net", Installation: installation})
}

// An inlineCard has to come back out of the editor as a bare URL on its own, or
// an edit that doesn't touch it would downgrade it to a plain link.
func TestCloudInteractiveEditKeepsSmartLinks(t *testing.T) {
	const url = "https://example.atlassian.net/browse/ABC-264"

	issue := &jira.Issue{Fields: jira.IssueFields{Description: &adf.ADF{
		Version: 1,
		DocType: "doc",
		Content: []*adf.Node{
			{
				NodeType: adf.NodeParagraph,
				Content: []*adf.Node{
					{NodeType: adf.ChildNodeText, NodeValue: adf.NodeValue{Text: "Blocked by"}},
					{NodeType: adf.InlineNodeCard, Attributes: map[string]any{"url": url}},
					{NodeType: adf.ChildNodeText, NodeValue: adf.NodeValue{Text: "until Friday"}},
				},
			},
		},
	}}}

	prefilled := editorBody(issue)
	assert.NotContains(t, prefilled, "📍")

	smart := "[" + url + "|" + url + "|smart-link]"

	edr := buildEditRequest(newClient(jira.InstallationTypeCloud), "TEST", issue, &editParams{body: prefilled})
	assert.Equal(t, "Blocked by "+smart+" until Friday", strings.Join(strings.Fields(edr.Body), " "))

	edr = buildEditRequest(newClient(jira.InstallationTypeLocal), "TEST", issue, &editParams{body: prefilled})
	assert.NotContains(t, edr.Body, "smart-link")
}

func TestSameBody(t *testing.T) {
	assert.True(t, sameBody("hi\n\n\n[attachment]", "hi\n\n\n[attachment]\n"))
	assert.True(t, sameBody("", ""))
	assert.False(t, sameBody("hi", "hi there"))
	assert.False(t, sameBody("  code", "code"))
}

func TestQuestionsSkipDescriptionEditorWhenItEmbedsMedia(t *testing.T) {
	withMedia := &adf.ADF{
		Version: 1,
		DocType: "doc",
		Content: []*adf.Node{
			{NodeType: adf.NodeMediaSingle, Content: []*adf.Node{{NodeType: adf.NodeMedia}}},
		},
	}

	tests := []struct {
		name        string
		description interface{}
		params      *editParams
		want        []string
	}{
		{name: "description embeds media", description: withMedia, params: &editParams{}, want: []string{"summary"}},
		{name: "override flag offers the editor", description: withMedia, params: &editParams{allowImageLoss: true}, want: []string{"summary", "body"}},
		{name: "no media", description: (*adf.ADF)(nil), params: &editParams{}, want: []string{"summary", "body"}},
		{name: "server wiki description", description: "!image.png!", params: &editParams{}, want: []string{"summary", "body"}},
		{name: "summary given, description embeds media", description: withMedia, params: &editParams{summary: "s"}, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issue := &jira.Issue{}
			issue.Fields.Description = tc.description
			ec := editCmd{params: tc.params}

			var got []string
			for _, q := range ec.questions(issue, "") {
				got = append(got, q.Name)
			}

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetMetadataQuestionsOriginalEstimateDefaultsToCurrent(t *testing.T) {
	issue := &jira.Issue{}
	issue.Fields.TimeTracking.OriginalEstimate = "1d 4h"

	qs := getMetadataQuestions([]string{metadataOriginalEstimate}, issue)

	if assert.Len(t, qs, 1) {
		assert.Equal(t, "originalestimate", qs[0].Name)
		assert.Equal(t, "1d 4h", qs[0].Prompt.(*survey.Input).Default)
	}
}

func TestApplyOriginalEstimateAnswer(t *testing.T) {
	issue := &jira.Issue{}
	issue.Fields.TimeTracking.OriginalEstimate = "1d"

	cases := []struct {
		name, flag, answer, want string
	}{
		{name: "new answer is used", answer: "3h", want: "3h"},
		{name: "answer wins over the flag", flag: "2h", answer: "3h", want: "3h"},
		{name: "confirming the current estimate sends nothing", flag: "2h", answer: "1d", want: ""},
		{name: "empty answer keeps the flag", flag: "2h", answer: "", want: "2h"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := &editParams{originalEstimate: tc.flag}

			applyOriginalEstimateAnswer(params, issue, tc.answer)

			assert.Equal(t, tc.want, params.originalEstimate)
		})
	}
}
