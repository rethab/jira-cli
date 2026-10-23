package edit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rethab/jira-cli/pkg/adf"
)

func TestEditorBody(t *testing.T) {
	t.Run("cloud body is translated from ADF", func(t *testing.T) {
		var doc adf.ADF
		require.NoError(t, json.Unmarshal([]byte(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[
			{"type":"text","text":"jira-cli smart link test, see "},
			{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/browse/KEY-1"}},
			{"type":"text","text":"): done"}
		]}]}`), &doc))

		assert.Equal(t, "jira-cli smart link test, see https://example.atlassian.net/browse/KEY-1 ): done\n\n", editorBody(&doc))
	})

	t.Run("server body is translated from wiki markup", func(t *testing.T) {
		assert.Equal(t, "some **bold**\n\n", editorBody("some *bold*"))
	})
}
