package md

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToJiraMDTrailingPunctuationStaysOutsideLink(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "closing parenthesis and colon",
			input:    "See https://example.atlassian.net/browse/ABC-264): done",
			expected: `See [https://example.atlassian.net/browse/ABC\-264|https://example.atlassian.net/browse/ABC-264]\): done`,
		},
		{
			name:     "closing parenthesis opened before the url",
			input:    "(see https://example.com/a)",
			expected: `\(see [https://example.com/a|https://example.com/a]\)`,
		},
		{
			name:     "full stop",
			input:    "Look at https://example.com/a.",
			expected: `Look at [https://example.com/a|https://example.com/a].`,
		},
		{
			name:     "several punctuation marks",
			input:    "Is it https://example.com/a?!;",
			expected: `Is it [https://example.com/a|https://example.com/a]?\!;`,
		},
		{
			name:     "url in a list item followed by a comma",
			input:    "- https://example.com/a, and more",
			expected: `* [https://example.com/a|https://example.com/a], and more`,
		},
		{
			name:     "explicit link keeps its destination",
			input:    "[see](https://example.com/a)",
			expected: `[see|https://example.com/a]`,
		},
		{
			name:     "parenthesis the url opens itself",
			input:    "See https://en.wikipedia.org/wiki/Pikachu_(Electric).",
			expected: `See [https://en.wikipedia.org/wiki/Pikachu\_\(Electric\)|https://en.wikipedia.org/wiki/Pikachu_(Electric)].`,
		},
		{
			name:     "parenthesis the url opens itself, inside parentheses",
			input:    "(https://en.wikipedia.org/wiki/Pikachu_(Electric))",
			expected: `\([https://en.wikipedia.org/wiki/Pikachu\_\(Electric\)|https://en.wikipedia.org/wiki/Pikachu_(Electric)]\)`,
		},
		{
			name:     "closing square bracket",
			input:    "[https://example.com/a]",
			expected: `\[[https://example.com/a|https://example.com/a]\]`,
		},
		{
			name:     "quotes the url opens itself",
			input:    "https://example.com/'quoted'",
			expected: `[https://example.com/'quoted'|https://example.com/'quoted']`,
		},
		{
			name:     "angle brackets keep a trailing full stop",
			input:    "<https://example.com/a.>",
			expected: `[https://example.com/a.|https://example.com/a.]`,
		},
		{
			name:     "explicit link keeps a trailing full stop",
			input:    "[https://example.com/a.](https://example.com/a.)",
			expected: `[https://example.com/a.|https://example.com/a.]`,
		},
		{
			name:     "url at the end of a line",
			input:    "see https://example.com/a):\nnext line",
			expected: "see [https://example.com/a|https://example.com/a]\\):\nnext line",
		},
		{
			name:     "url in a heading",
			input:    "# See https://example.com/a.",
			expected: `h1. See [https://example.com/a|https://example.com/a].`,
		},
		{
			name:     "url in a table",
			input:    "| a | b |\n|---|---|\n| https://example.com/a): | x |",
			expected: "||a||b||\n|[https://example.com/a|https://example.com/a]\\):|x|",
		},
		{
			name:     "code span is untouched",
			input:    "`https://example.com/a):`",
			expected: `{{https://example.com/a\):}}`,
		},
		{
			name:     "code block is untouched",
			input:    "```\nhttps://example.com/a):\n```",
			expected: "{code}\nhttps://example.com/a):\n{code}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ToJiraMD(tc.input))
		})
	}
}

func TestToJiraMDSmartLinks(t *testing.T) {
	const server = "https://example.atlassian.net"
	const smart = "[https://example.atlassian.net/browse/ABC-264|https://example.atlassian.net/browse/ABC-264|smart-link]"

	tests := []struct {
		name     string
		input    string
		server   string
		expected string
	}{
		{
			name:     "bare issue url",
			input:    "https://example.atlassian.net/browse/ABC-264",
			server:   server,
			expected: smart,
		},
		{
			name:     "within a sentence",
			input:    "Blocked by https://example.atlassian.net/browse/ABC-264 until Friday",
			server:   server,
			expected: "Blocked by " + smart + " until Friday",
		},
		{
			name:     "punctuation stays outside",
			input:    "See https://example.atlassian.net/browse/ABC-264): done",
			server:   server,
			expected: `See ` + smart + `\): done`,
		},
		{
			name:     "trailing slash on the configured server",
			input:    "https://example.atlassian.net/browse/ABC-264",
			server:   server + "/",
			expected: smart,
		},
		{
			name:     "host is compared case-insensitively",
			input:    "https://EXAMPLE.atlassian.net/browse/ABC-264",
			server:   server,
			expected: "[https://EXAMPLE.atlassian.net/browse/ABC-264|https://EXAMPLE.atlassian.net/browse/ABC-264|smart-link]",
		},
		{
			name:     "project key with digits and underscore",
			input:    "https://example.atlassian.net/browse/A2_B-1",
			server:   server,
			expected: "[https://example.atlassian.net/browse/A2_B-1|https://example.atlassian.net/browse/A2_B-1|smart-link]",
		},
		{
			name:     "angle bracket autolink",
			input:    "<https://example.atlassian.net/browse/ABC-264>",
			server:   server,
			expected: smart,
		},
		{
			name:     "inside a list item",
			input:    "- see https://example.atlassian.net/browse/ABC-264",
			server:   server,
			expected: "* see " + smart,
		},
		{
			name:   "several in one paragraph",
			input:  "https://example.atlassian.net/browse/ABC-1 and https://example.atlassian.net/browse/ABC-2",
			server: server,
			expected: "[https://example.atlassian.net/browse/ABC-1|https://example.atlassian.net/browse/ABC-1|smart-link]" +
				" and " +
				"[https://example.atlassian.net/browse/ABC-2|https://example.atlassian.net/browse/ABC-2|smart-link]",
		},
		{
			name:     "issue on another host stays a regular link",
			input:    "https://other.atlassian.net/browse/ABC-264",
			server:   server,
			expected: `[https://other.atlassian.net/browse/ABC\-264|https://other.atlassian.net/browse/ABC-264]`,
		},
		{
			name:     "host that merely starts with the server stays a regular link",
			input:    "https://example.atlassian.net.evil.com/browse/ABC-264",
			server:   server,
			expected: `[https://example.atlassian.net.evil.com/browse/ABC\-264|https://example.atlassian.net.evil.com/browse/ABC-264]`,
		},
		{
			name:     "other page on the same site stays a regular link",
			input:    "https://example.atlassian.net/wiki/spaces/ANK/pages/1",
			server:   server,
			expected: `[https://example.atlassian.net/wiki/spaces/ANK/pages/1|https://example.atlassian.net/wiki/spaces/ANK/pages/1]`,
		},
		{
			name:     "issue url with a query stays a regular link",
			input:    "https://example.atlassian.net/browse/ABC-264?focusedCommentId=1",
			server:   server,
			expected: `[https://example.atlassian.net/browse/ABC\-264?focusedCommentId=1|https://example.atlassian.net/browse/ABC-264?focusedCommentId=1]`,
		},
		{
			name:     "explicit link with its own text stays a regular link",
			input:    "[the bug](https://example.atlassian.net/browse/ABC-264)",
			server:   server,
			expected: `[the bug|https://example.atlassian.net/browse/ABC-264]`,
		},
		{
			name:     "without smart links configured",
			input:    "https://example.atlassian.net/browse/ABC-264",
			server:   "",
			expected: `[https://example.atlassian.net/browse/ABC\-264|https://example.atlassian.net/browse/ABC-264]`,
		},
		{
			name:     "mentions and smart links together",
			input:    "[~ankit] see https://example.atlassian.net/browse/ABC-264",
			server:   server,
			expected: "[~ankit] see " + smart,
		},
		{
			name:     "other scheme stays a regular link",
			input:    "http://example.atlassian.net/browse/ABC-264",
			server:   server,
			expected: `[http://example.atlassian.net/browse/ABC\-264|http://example.atlassian.net/browse/ABC-264]`,
		},
		{
			name:     "in a heading",
			input:    "# Blocked by https://example.atlassian.net/browse/ABC-264",
			server:   server,
			expected: "h1. Blocked by " + smart,
		},
		{
			name:     "in a table",
			input:    "| issue |\n|---|\n| https://example.atlassian.net/browse/ABC-264 |",
			server:   server,
			expected: "||issue||\n|" + smart + "|",
		},
		{
			name:     "in a code span",
			input:    "`https://example.atlassian.net/browse/ABC-264`",
			server:   server,
			expected: `{{https://example.atlassian.net/browse/ABC\-264}}`,
		},
		{
			name:     "in a code block",
			input:    "```\nhttps://example.atlassian.net/browse/ABC-264\n```",
			server:   server,
			expected: "{code}\nhttps://example.atlassian.net/browse/ABC-264\n{code}",
		},
		{
			name:     "block separation after a smart link",
			input:    "https://example.atlassian.net/browse/ABC-264\n\n# Heading",
			server:   server,
			expected: smart + "\n\nh1. Heading",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var opts []Option
			if tc.server != "" {
				opts = append(opts, WithSmartLinks(tc.server))
			}
			assert.Equal(t, tc.expected, ToJiraMD(tc.input, opts...))
		})
	}
}
