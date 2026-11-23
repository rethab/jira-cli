package adf

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaIDs(t *testing.T) {
	const withMedia = `{"version":1,"type":"doc","content":[
		{"type":"mediaSingle","content":[
			{"type":"media","attrs":{"id":"d3a1a2f4-0000-4000-8000-000000000001","type":"file"}}]},
		{"type":"paragraph","content":[
			{"type":"mediaInline","attrs":{"id":"d3a1a2f4-0000-4000-8000-000000000002","type":"file"}}]},
		{"type":"mediaGroup","content":[
			{"type":"media","attrs":{"id":"d3a1a2f4-0000-4000-8000-000000000003","type":"file"}},
			{"type":"media","attrs":{"id":"d3a1a2f4-0000-4000-8000-000000000004","type":"file"}}]},
		{"type":"mediaSingle","content":[
			{"type":"media","attrs":{"type":"external","url":"https://example.atlassian.net/image.png"}}]},
		{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[
			{"type":"mediaSingle","content":[
				{"type":"media","attrs":{"id":"d3a1a2f4-0000-4000-8000-000000000005","type":"file"}}]}]}]}]}
	]}`

	var doc ADF
	require.NoError(t, json.Unmarshal([]byte(withMedia), &doc))

	assert.Equal(t, []string{
		"d3a1a2f4-0000-4000-8000-000000000001",
		"d3a1a2f4-0000-4000-8000-000000000002",
		"d3a1a2f4-0000-4000-8000-000000000003",
		"d3a1a2f4-0000-4000-8000-000000000004",
		"d3a1a2f4-0000-4000-8000-000000000005",
	}, doc.MediaIDs())

	t.Run("nil document", func(t *testing.T) {
		var nilDoc *ADF
		assert.Empty(t, nilDoc.MediaIDs())
	})

	t.Run("no media", func(t *testing.T) {
		plain := &ADF{Version: 1, DocType: "doc", Content: []*Node{
			{NodeType: NodeParagraph, Content: []*Node{{NodeType: ChildNodeText, NodeValue: NodeValue{Text: "hi"}}}},
		}}
		assert.Empty(t, plain.MediaIDs())
	})
}
