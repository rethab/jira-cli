package view

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
)

func TestIssueCommentsRenderADF(t *testing.T) {
	var doc adf.ADF
	require.NoError(t, json.Unmarshal([]byte(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[
		{"type":"text","text":"jira-cli smart link test, see "},
		{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/browse/KEY-1"}},
		{"type":"text","text":"): done"}
	]}]}`), &doc))

	var b bytes.Buffer
	c := NewIssueComments([]jira.Comment{{
		ID:      "10",
		Author:  jira.User{DisplayName: "Person A"},
		Created: "2026-01-14T09:00:00.000+0000",
		Updated: "2026-01-14T09:00:00.000+0000",
		Body:    &doc,
	}}, ListDisplayFormat{}, WithListWriter(&b))
	require.NoError(t, c.Render())

	expected := `ID	AUTHOR	CREATED	UPDATED	BODY
10	Person A	2026-01-14 09:00:00	2026-01-14 09:00:00	jira-cli smart link test, see https://example.atlassian.net/browse/KEY-1 ): done
`
	assert.Equal(t, expected, b.String())
}

func TestIssueCommentsRender(t *testing.T) {
	var b bytes.Buffer

	data := []jira.Comment{
		{
			ID:      "10",
			Author:  jira.User{DisplayName: "Person A"},
			Created: "2026-01-14T09:00:00.000+0000",
			Updated: "2026-01-14T09:00:00.000+0000",
			Body:    "first",
		},
		{
			ID:      "11",
			Author:  jira.User{Name: "person.b"},
			Created: "2026-01-15T09:00:00.000+0000",
			Updated: "2026-01-16T10:30:00.000+0000",
			Body:    "some *bold*\nmulti-line",
		},
	}

	c := NewIssueComments(data, ListDisplayFormat{}, WithListWriter(&b))
	assert.NoError(t, c.Render())

	expected := `ID	AUTHOR	CREATED	UPDATED	BODY
10	Person A	2026-01-14 09:00:00	2026-01-14 09:00:00	first
11	person.b	2026-01-15 09:00:00	2026-01-16 10:30:00	some **bold** multi-line
`
	assert.Equal(t, expected, b.String())
}

func TestIssueAttachmentsRender(t *testing.T) {
	var b bytes.Buffer

	data := []jira.Attachment{
		{ID: "10", Filename: "a.txt", Size: 12, Author: jira.User{DisplayName: "Person A"}, Created: "2026-01-14T09:00:00.000+0000"},
		{ID: "11", Filename: "b.png", Size: 5 * 1024 * 1024, Author: jira.User{Name: "person.b"}, Created: "2026-01-15T09:00:00.000+0000"},
	}

	a := NewIssueAttachments(data, ListDisplayFormat{}, WithListWriter(&b))
	assert.NoError(t, a.Render())

	expected := `ID	FILENAME	SIZE	AUTHOR	CREATED
10	a.txt	12 B	Person A	2026-01-14 09:00:00
11	b.png	5.0 MB	person.b	2026-01-15 09:00:00
`
	assert.Equal(t, expected, b.String())
}

func TestFormatSize(t *testing.T) {
	assert.Equal(t, "0 B", formatSize(0))
	assert.Equal(t, "1023 B", formatSize(1023))
	assert.Equal(t, "1.0 KB", formatSize(1024))
	assert.Equal(t, "1.5 KB", formatSize(1536))
	assert.Equal(t, "1.0 MB", formatSize(1024*1024))
	assert.Equal(t, "2.0 GB", formatSize(2*1024*1024*1024))
	assert.Equal(t, "2048.0 GB", formatSize(2048*1024*1024*1024))
}
