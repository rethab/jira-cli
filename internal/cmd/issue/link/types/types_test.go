package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func TestFormat(t *testing.T) {
	linkTypes := []*jira.IssueLinkType{
		{ID: "10000", Name: "Blocks", Outward: "blocks", Inward: "is blocked by"},
		{ID: "10001", Name: "Relates", Outward: "relates to", Inward: "relates to"},
	}

	t.Run("plain", func(t *testing.T) {
		assert.Equal(t, "Blocks\tblocks\tis blocked by\nRelates\trelates to\trelates to", format(linkTypes, true))
	})

	t.Run("pretty", func(t *testing.T) {
		assert.Equal(
			t,
			"NAME     OUTWARD     INWARD\nBlocks   blocks      is blocked by\nRelates  relates to  relates to",
			format(linkTypes, false),
		)
	})

	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, format(nil, false))
		assert.Empty(t, format(nil, true))
	})
}
