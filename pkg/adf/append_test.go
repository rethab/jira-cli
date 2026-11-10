package adf

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func listOf(t NodeType, attrs any, items ...string) *Node {
	n := &Node{NodeType: t, Attributes: attrs}
	for _, item := range items {
		n.Content = append(n.Content, &Node{NodeType: ChildNodeListItem, Content: []*Node{paragraphOf(item)}})
	}
	return n
}

func paragraphOf(text string) *Node {
	return &Node{NodeType: NodeParagraph, Content: []*Node{{NodeType: ChildNodeText, NodeValue: NodeValue{Text: text}}}}
}

func docOf(nodes ...*Node) *ADF {
	return &ADF{Version: 1, DocType: "doc", Content: nodes}
}

func TestAppend(t *testing.T) {
	tests := []struct {
		name     string
		existing *ADF
		added    *ADF
		want     *ADF
	}{
		{
			name:     "nil receiver yields the appended document",
			existing: nil,
			added:    docOf(paragraphOf("new")),
			want:     docOf(paragraphOf("new")),
		},
		{
			name:     "empty content gets the appended nodes",
			existing: docOf(),
			added:    docOf(paragraphOf("new")),
			want:     docOf(paragraphOf("new")),
		},
		{
			name:     "paragraphs are added after the existing nodes",
			existing: docOf(paragraphOf("old")),
			added:    docOf(paragraphOf("new")),
			want:     docOf(paragraphOf("old"), paragraphOf("new")),
		},
		{
			name:     "bullet list joins a trailing bullet list and keeps its attrs",
			existing: docOf(paragraphOf("old"), listOf(NodeBulletList, map[string]any{"localId": "x"}, "a", "b")),
			added:    docOf(listOf(NodeBulletList, nil, "c"), paragraphOf("after")),
			want: docOf(
				paragraphOf("old"),
				listOf(NodeBulletList, map[string]any{"localId": "x"}, "a", "b", "c"),
				paragraphOf("after"),
			),
		},
		{
			name:     "ordered list joins a trailing ordered list",
			existing: docOf(listOf(NodeOrderedList, map[string]any{"order": float64(3)}, "a")),
			added:    docOf(listOf(NodeOrderedList, nil, "b")),
			want:     docOf(listOf(NodeOrderedList, map[string]any{"order": float64(3)}, "a", "b")),
		},
		{
			name:     "different list kinds stay separate",
			existing: docOf(listOf(NodeBulletList, nil, "a")),
			added:    docOf(listOf(NodeOrderedList, nil, "b")),
			want:     docOf(listOf(NodeBulletList, nil, "a"), listOf(NodeOrderedList, nil, "b")),
		},
		{
			name:     "list after a paragraph stays separate",
			existing: docOf(paragraphOf("old")),
			added:    docOf(listOf(NodeBulletList, nil, "b")),
			want:     docOf(paragraphOf("old"), listOf(NodeBulletList, nil, "b")),
		},
		{
			name:     "paragraph after a list leaves the list untouched",
			existing: docOf(listOf(NodeBulletList, nil, "a")),
			added:    docOf(paragraphOf("new")),
			want:     docOf(listOf(NodeBulletList, nil, "a"), paragraphOf("new")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.existing.Append(tc.added)

			want, err := json.Marshal(tc.want)
			assert.NoError(t, err)
			have, err := json.Marshal(got)
			assert.NoError(t, err)
			assert.JSONEq(t, string(want), string(have))
		})
	}
}

func TestPrepend(t *testing.T) {
	media := func() *Node {
		return &Node{
			NodeType:   NodeMediaSingle,
			Attributes: map[string]any{"layout": "center", "width": float64(50)},
			Content: []*Node{{
				NodeType:   NodeMedia,
				Attributes: map[string]any{"id": "d3a1a2f4-0000-4000-8000-000000000001", "type": "file", "collection": ""},
			}},
		}
	}
	heading := &Node{
		NodeType:   NodeHeading,
		Attributes: map[string]any{"level": float64(2)},
		Content:    []*Node{{NodeType: ChildNodeText, NodeValue: NodeValue{Text: "Status"}}},
	}

	tests := []struct {
		name     string
		existing *ADF
		added    *ADF
		want     *ADF
	}{
		{
			name:     "nil receiver yields the prepended document",
			existing: nil,
			added:    docOf(paragraphOf("new")),
			want:     docOf(paragraphOf("new")),
		},
		{
			name:     "empty content gets the prepended nodes",
			existing: docOf(),
			added:    docOf(paragraphOf("new")),
			want:     docOf(paragraphOf("new")),
		},
		{
			name:     "nothing to prepend leaves the document as it is",
			existing: docOf(paragraphOf("old")),
			added:    docOf(),
			want:     docOf(paragraphOf("old")),
		},
		{
			name:     "paragraphs are added before the existing nodes",
			existing: docOf(paragraphOf("old")),
			added:    docOf(paragraphOf("new")),
			want:     docOf(paragraphOf("new"), paragraphOf("old")),
		},
		{
			name:     "media stays untouched",
			existing: docOf(media(), paragraphOf("old")),
			added:    docOf(heading, paragraphOf("new")),
			want:     docOf(heading, paragraphOf("new"), media(), paragraphOf("old")),
		},
		{
			name:     "trailing bullet list joins a leading bullet list and keeps its attrs",
			existing: docOf(listOf(NodeBulletList, map[string]any{"localId": "x"}, "a", "b"), paragraphOf("after")),
			added:    docOf(paragraphOf("before"), listOf(NodeBulletList, nil, "z")),
			want: docOf(
				paragraphOf("before"),
				listOf(NodeBulletList, map[string]any{"localId": "x"}, "z", "a", "b"),
				paragraphOf("after"),
			),
		},
		{
			name:     "ordered list joins a leading ordered list and keeps its attrs",
			existing: docOf(listOf(NodeOrderedList, map[string]any{"order": float64(3)}, "a")),
			added:    docOf(listOf(NodeOrderedList, nil, "z")),
			want:     docOf(listOf(NodeOrderedList, map[string]any{"order": float64(3)}, "z", "a")),
		},
		{
			name:     "different list kinds stay separate",
			existing: docOf(listOf(NodeBulletList, nil, "a")),
			added:    docOf(listOf(NodeOrderedList, nil, "z")),
			want:     docOf(listOf(NodeOrderedList, nil, "z"), listOf(NodeBulletList, nil, "a")),
		},
		{
			name:     "list before a paragraph stays separate",
			existing: docOf(paragraphOf("old")),
			added:    docOf(listOf(NodeBulletList, nil, "z")),
			want:     docOf(listOf(NodeBulletList, nil, "z"), paragraphOf("old")),
		},
		{
			name:     "paragraph before a list leaves the list untouched",
			existing: docOf(listOf(NodeBulletList, nil, "a")),
			added:    docOf(paragraphOf("new")),
			want:     docOf(paragraphOf("new"), listOf(NodeBulletList, nil, "a")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.existing.Prepend(tc.added)

			want, err := json.Marshal(tc.want)
			assert.NoError(t, err)
			have, err := json.Marshal(got)
			assert.NoError(t, err)
			assert.JSONEq(t, string(want), string(have))
		})
	}

	t.Run("does not mutate its inputs", func(t *testing.T) {
		existing := docOf(listOf(NodeBulletList, nil, "a", "b"))
		added := docOf(listOf(NodeBulletList, nil, "z"))

		got := existing.Prepend(added)

		assert.Len(t, got.Content[0].Content, 3)
		assert.Len(t, existing.Content[0].Content, 2)
		assert.Len(t, added.Content[0].Content, 1)
	})
}
