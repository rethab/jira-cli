package link

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

var blocks = &jira.IssueLinkType{ID: "10000", Name: "Blocks", Outward: "blocks", Inward: "is blocked by"}

func TestLinkTypeOption(t *testing.T) {
	assert.Equal(t, "Blocks: blocks", linkTypeOption(blocks))
}

func TestLinkedMessage(t *testing.T) {
	assert.Equal(t, "Linked: TEST-1 blocks TEST-2", linkedMessage("TEST-1", "TEST-2", blocks))
}

func TestRouting(t *testing.T) {
	link := NewCmdLink()
	root := &cobra.Command{Use: "jira"}
	issue := &cobra.Command{Use: "issue"}
	root.AddCommand(issue)
	issue.AddCommand(link)

	t.Run("types subcommand", func(t *testing.T) {
		cmd, _, err := root.Find([]string{"issue", "link", "types"})

		assert.NoError(t, err)
		assert.Equal(t, "types", cmd.Name())
	})

	t.Run("positional form", func(t *testing.T) {
		cmd, args, err := root.Find([]string{"issue", "link", "TEST-1", "TEST-2", "Blocks"})

		assert.NoError(t, err)
		assert.Same(t, link, cmd)
		assert.Equal(t, []string{"TEST-1", "TEST-2", "Blocks"}, args)
	})
}
