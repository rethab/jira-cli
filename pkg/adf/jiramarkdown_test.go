package adf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJiraMarkdownInlineCardIsABareURL(t *testing.T) {
	doc := ADF{
		Version: 1,
		DocType: "doc",
		Content: []*Node{
			{
				NodeType: NodeParagraph,
				Content: []*Node{
					{NodeType: ChildNodeText, NodeValue: NodeValue{Text: "See"}},
					{NodeType: InlineNodeCard, Attributes: map[string]any{"url": "https://example.atlassian.net/browse/ABC-1"}},
				},
			},
		},
	}

	out := NewTranslator(&doc, NewJiraMarkdownTranslator()).Translate()

	assert.Contains(t, out, "See https://example.atlassian.net/browse/ABC-1")
	assert.NotContains(t, out, "📍")
}
