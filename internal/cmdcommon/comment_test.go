package cmdcommon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/md"
)

func TestEditorBody(t *testing.T) {
	t.Run("cloud body is translated from ADF", func(t *testing.T) {
		var doc adf.ADF
		require.NoError(t, json.Unmarshal([]byte(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[
			{"type":"text","text":"jira-cli smart link test, see "},
			{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/browse/KEY-1"}},
			{"type":"text","text":"): done"}
		]}]}`), &doc))

		assert.Equal(t, "jira-cli smart link test, see https://example.atlassian.net/browse/KEY-1 ): done\n\n", EditorBody(&doc))
	})

	t.Run("server body is translated from wiki markup", func(t *testing.T) {
		assert.Equal(t, "some **bold**\n\n", EditorBody("some *bold*"))
	})
}

func TestEditorBodyKeepsMentions(t *testing.T) {
	t.Run("cloud", func(t *testing.T) {
		var doc adf.ADF
		require.NoError(t, json.Unmarshal([]byte(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[
			{"type":"text","text":"Thanks "},
			{"type":"mention","attrs":{"id":"5b10a2844c20165700ede21g","text":"@Jane Doe"}},
			{"type":"text","text":" for looking"}
		]}]}`), &doc))

		assert.Equal(t, "Thanks [~accountid:5b10a2844c20165700ede21g] for looking\n\n", md.ToJiraMD(EditorBody(&doc))+"\n\n")
	})

	t.Run("server", func(t *testing.T) {
		wiki := "Thanks [~jdoe] for looking"

		assert.Equal(t, wiki, md.ToJiraMD(EditorBody(wiki)))
	})
}

func TestReplyPrefix(t *testing.T) {
	author := jira.User{AccountID: "acc-1", Name: "jdoe", DisplayName: "Jane Doe"}

	cases := []struct {
		name     string
		author   jira.User
		body     any
		local    bool
		expected string
	}{
		{
			name:     "cloud quotes the body",
			author:   author,
			body:     "Looks good to me",
			expected: "[~accountid:acc-1] wrote:\n\n> Looks good to me\n\n",
		},
		{
			name:     "local mentions the username",
			author:   author,
			body:     "Looks good to me",
			local:    true,
			expected: "[~jdoe] wrote:\n\n> Looks good to me\n\n",
		},
		{
			name:     "empty body only mentions",
			author:   author,
			body:     "",
			expected: "[~accountid:acc-1]\n\n",
		},
		{
			name:     "first three lines, ellipsis when more follow",
			author:   author,
			body:     "one\n\ntwo\nthree\nfour\nfive",
			expected: "[~accountid:acc-1] wrote:\n\n> one\n> two\n> three…\n\n",
		},
		{
			name:     "exactly three lines are not cut",
			author:   author,
			body:     "one\ntwo\nthree",
			expected: "[~accountid:acc-1] wrote:\n\n> one\n> two\n> three\n\n",
		},
		{
			name:     "long text is truncated",
			author:   author,
			body:     strings.Repeat("a", 400),
			expected: "[~accountid:acc-1] wrote:\n\n> " + strings.Repeat("a", 300) + "…\n\n",
		},
		{
			name:     "budget is shared between lines",
			author:   author,
			body:     strings.Repeat("a", 200) + "\n" + strings.Repeat("b", 200),
			expected: "[~accountid:acc-1] wrote:\n\n> " + strings.Repeat("a", 200) + "\n> " + strings.Repeat("b", 100) + "…\n\n",
		},
		{
			name:     "multibyte text is cut on a character",
			author:   author,
			body:     strings.Repeat("é", 301),
			expected: "[~accountid:acc-1] wrote:\n\n> " + strings.Repeat("é", 300) + "…\n\n",
		},
		{
			name:     "code fences are not quoted",
			author:   author,
			body:     "{code}\ncode\n{code}",
			expected: "[~accountid:acc-1] wrote:\n\n> code\n\n",
		},
		{
			name:     "a line that uses up the budget takes the ellipsis",
			author:   author,
			body:     strings.Repeat("a", 300) + "\nsecond",
			expected: "[~accountid:acc-1] wrote:\n\n> " + strings.Repeat("a", 300) + "…\n\n",
		},
		{
			name:     "a link cut in half is dropped",
			author:   author,
			body:     strings.Repeat("a", 290) + " [docs|https://example.com/a/long/path]",
			expected: "[~accountid:acc-1] wrote:\n\n> " + strings.Repeat("a", 290) + "…\n\n",
		},
		{
			name:     "a link cut in its target is dropped",
			author:   author,
			body:     strings.Repeat("a", 290) + " [docs|https://example.com/page]",
			expected: "[~accountid:acc-1] wrote:\n\n> " + strings.Repeat("a", 290) + "…\n\n",
		},
		{
			name:     "a mention before the cut is quoted as plain text",
			author:   author,
			body:     "[~bob] " + strings.Repeat("a", 300),
			expected: "[~accountid:acc-1] wrote:\n\n> @bob " + strings.Repeat("a", 295) + "…\n\n",
		},
		{
			name:     "author without an id is named",
			author:   jira.User{DisplayName: "Gone User"},
			body:     "bye",
			expected: "Gone User wrote:\n\n> bye\n\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReplyPrefix(&jira.Comment{Author: tc.author, Body: tc.body}, tc.local)

			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestReplyPrefixOfACloudComment(t *testing.T) {
	var doc adf.ADF
	require.NoError(t, json.Unmarshal([]byte(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[
		{"type":"text","text":"Please see "},
		{"type":"mention","attrs":{"id":"acc-2","text":"@Bob"}}
	]}]}`), &doc))

	got := ReplyPrefix(&jira.Comment{Author: jira.User{AccountID: "acc-1"}, Body: &doc}, false)

	assert.Equal(t, "[~accountid:acc-1] wrote:\n\n> Please see @Bob\n\n", got)
}

func TestReplyPrefixOfAServerCommentDoesNotPingMentions(t *testing.T) {
	got := ReplyPrefix(&jira.Comment{Author: jira.User{Name: "jdoe"}, Body: "Please see [~bob]"}, true)

	assert.Equal(t, "[~jdoe] wrote:\n\n> Please see @bob\n\n", got)
}

func TestReplyPrefixSkipsPanelMacros(t *testing.T) {
	var doc adf.ADF
	require.NoError(t, json.Unmarshal([]byte(`{"type":"doc","version":1,"content":[{"type":"panel","attrs":{"panelType":"info"},"content":[
		{"type":"paragraph","content":[{"type":"text","text":"one"}]},
		{"type":"paragraph","content":[{"type":"text","text":"two"}]},
		{"type":"paragraph","content":[{"type":"text","text":"three"}]},
		{"type":"paragraph","content":[{"type":"text","text":"four"}]}
	]}]}`), &doc))

	got := ReplyPrefix(&jira.Comment{Author: jira.User{AccountID: "acc-1"}, Body: &doc}, false)

	assert.Equal(t, "[~accountid:acc-1] wrote:\n\n> one\n> two\n> three…\n\n", got)
}

func TestEditorBodyEscapesLiteralMentionSyntax(t *testing.T) {
	var doc adf.ADF
	require.NoError(t, json.Unmarshal([]byte(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[
		{"type":"text","text":"write @[name] to mention"}
	]}]}`), &doc))

	assert.Equal(t, "write @\\[name] to mention\n\n", EditorBody(&doc))
	assert.Equal(t, "write @\\[name] to mention", EditorBody(`write @\[name\] to mention`))
}

func TestReplyPrefixDoesNotResolveQuotedMentionSyntax(t *testing.T) {
	var doc adf.ADF
	require.NoError(t, json.Unmarshal([]byte(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[
		{"type":"text","text":"write @[name]"}
	]}]}`), &doc))

	got := ReplyPrefix(&jira.Comment{Author: jira.User{AccountID: "acc-1"}, Body: &doc}, false)

	assert.Equal(t, "[~accountid:acc-1] wrote:\n\n> write @\\[name]\n\n", got)
}
