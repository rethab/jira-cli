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
