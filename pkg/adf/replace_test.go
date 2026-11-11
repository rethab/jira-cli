package adf

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func textOf(text string, marks ...MarkNode) *Node {
	return &Node{NodeType: ChildNodeText, NodeValue: NodeValue{Text: text, Marks: marks}}
}

func parentOf(t NodeType, children ...*Node) *Node {
	return &Node{NodeType: t, Content: children}
}

func marshal(t *testing.T, a *ADF) string {
	t.Helper()
	b, err := json.Marshal(a)
	require.NoError(t, err)
	return string(b)
}

func TestReplaceText(t *testing.T) {
	strong := MarkNode{MarkType: MarkStrong}
	link := MarkNode{MarkType: MarkLink, Attributes: map[string]any{"href": "https://example.atlassian.net"}}

	tests := []struct {
		name      string
		doc       *ADF
		old, new  string
		wantCount int
		want      *ADF
	}{
		{
			name:      "plain paragraph counts every occurrence",
			doc:       docOf(paragraphOf("foo and foo")),
			old:       "foo",
			new:       "bar",
			wantCount: 2,
			want:      docOf(paragraphOf("bar and bar")),
		},
		{
			name: "marks are kept",
			doc: docOf(parentOf(NodeParagraph,
				textOf("see foo", strong, link),
			)),
			old:       "foo",
			new:       "bar",
			wantCount: 1,
			want: docOf(parentOf(NodeParagraph,
				textOf("see bar", strong, link),
			)),
		},
		{
			name: "nested text in table, panel and expand",
			doc: docOf(
				parentOf(NodeTable, parentOf(ChildNodeTableRow, parentOf(ChildNodeTableCell, paragraphOf("foo")))),
				parentOf(NodePanel, paragraphOf("foo")),
				parentOf(NodeType("expand"), paragraphOf("foo")),
			),
			old:       "foo",
			new:       "bar",
			wantCount: 3,
			want: docOf(
				parentOf(NodeTable, parentOf(ChildNodeTableRow, parentOf(ChildNodeTableCell, paragraphOf("bar")))),
				parentOf(NodePanel, paragraphOf("bar")),
				parentOf(NodeType("expand"), paragraphOf("bar")),
			),
		},
		{
			name: "does not match across text nodes",
			doc: docOf(parentOf(NodeParagraph,
				textOf("foo "),
				textOf("bar", strong),
			)),
			old:       "foo bar",
			new:       "x",
			wantCount: 0,
			want: docOf(parentOf(NodeParagraph,
				textOf("foo "),
				textOf("bar", strong),
			)),
		},
		{
			name: "text node that becomes empty is removed",
			doc: docOf(parentOf(NodeParagraph,
				textOf("keep "),
				textOf("drop", strong),
			)),
			old:       "drop",
			new:       "",
			wantCount: 1,
			want:      docOf(parentOf(NodeParagraph, textOf("keep "))),
		},
		{
			name:      "paragraph stays when all its text is removed",
			doc:       docOf(paragraphOf("drop")),
			old:       "drop",
			new:       "",
			wantCount: 1,
			want:      docOf(&Node{NodeType: NodeParagraph}),
		},
		{
			name:      "empty search changes nothing",
			doc:       docOf(paragraphOf("foo")),
			old:       "",
			new:       "bar",
			wantCount: 0,
			want:      docOf(paragraphOf("foo")),
		},
		{
			name:      "empty document",
			doc:       docOf(),
			old:       "foo",
			new:       "bar",
			wantCount: 0,
			want:      docOf(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantCount, tc.doc.ReplaceText(tc.old, tc.new))
			assert.JSONEq(t, marshal(t, tc.want), marshal(t, tc.doc))
		})
	}
}

func TestReplaceTextNilReceiver(t *testing.T) {
	var a *ADF
	assert.Zero(t, a.ReplaceText("foo", "bar"))
}

func TestReplaceTextLeavesNonTextNodesAlone(t *testing.T) {
	media := parentOf(NodeMediaSingle, &Node{
		NodeType:   NodeMedia,
		Attributes: map[string]any{"id": "d3a1a2f4-0000-4000-8000-000000000001", "type": "file", "collection": ""},
	})
	card := &Node{NodeType: InlineNodeCard, Attributes: map[string]any{"url": "https://example.atlassian.net/browse/TEST-2"}}
	mention := &Node{
		NodeType:   InlineNodeMention,
		Attributes: map[string]any{"id": "abc123", "text": "@Jane Doe"},
	}
	build := func(text string) *ADF {
		return docOf(media, parentOf(NodeParagraph, textOf("Hi "), mention, textOf(" see "), card, textOf(text)))
	}

	t.Run("targeted text only", func(t *testing.T) {
		doc := build(" on Friday")
		assert.Equal(t, 1, doc.ReplaceText("Friday", "Monday"))
		assert.JSONEq(t, marshal(t, build(" on Monday")), marshal(t, doc))
	})

	t.Run("mention attrs are not text", func(t *testing.T) {
		doc := build(" on Friday")
		assert.Zero(t, doc.ReplaceText("Jane", "John"))
		assert.JSONEq(t, marshal(t, build(" on Friday")), marshal(t, doc))
	})
}
