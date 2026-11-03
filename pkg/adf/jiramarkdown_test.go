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

func mentionDoc(attrs map[string]any) *ADF {
	return &ADF{
		Version: 1,
		DocType: "doc",
		Content: []*Node{
			{
				NodeType: NodeParagraph,
				Content: []*Node{
					{NodeType: ChildNodeText, NodeValue: NodeValue{Text: "Thanks"}},
					{NodeType: InlineNodeMention, Attributes: attrs},
					{NodeType: ChildNodeText, NodeValue: NodeValue{Text: "for looking"}},
				},
			},
		},
	}
}

func TestJiraMarkdownMentionKeepsTheAccountID(t *testing.T) {
	doc := mentionDoc(map[string]any{"id": "5b10a2844c20165700ede21g", "text": "@Jane Doe"})

	out := NewTranslator(doc, NewJiraMarkdownTranslator()).Translate()

	assert.Contains(t, out, "Thanks [~accountid:5b10a2844c20165700ede21g] for looking")
	assert.NotContains(t, out, "Jane")
}

func TestJiraMarkdownMentionWithoutIDFallsBackToTheName(t *testing.T) {
	doc := mentionDoc(map[string]any{"text": "@Jane Doe"})

	out := NewTranslator(doc, NewJiraMarkdownTranslator()).Translate()

	assert.Contains(t, out, "Thanks @Jane Doe for looking")
}

func TestMarkdownMentionStaysTheName(t *testing.T) {
	doc := mentionDoc(map[string]any{"id": "5b10a2844c20165700ede21g", "text": "@Jane Doe"})

	out := NewTranslator(doc, NewMarkdownTranslator()).Translate()

	assert.Contains(t, out, "@Jane Doe")
	assert.NotContains(t, out, "accountid")
}
