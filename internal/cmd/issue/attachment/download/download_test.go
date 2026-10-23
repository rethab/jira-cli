package download

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func TestSelectAttachments(t *testing.T) {
	all := []jira.Attachment{
		{ID: "10", Filename: "a.txt"},
		{ID: "11", Filename: "b.txt"},
		{ID: "12", Filename: "a.txt"},
	}

	t.Run("all without refs", func(t *testing.T) {
		out, err := selectAttachments(all, nil)
		assert.NoError(t, err)
		assert.Equal(t, all, out)
	})

	t.Run("by id and by name", func(t *testing.T) {
		out, err := selectAttachments(all, []string{"11", "a.txt"})
		assert.NoError(t, err)
		assert.Equal(t, []jira.Attachment{all[1], all[0], all[2]}, out)
	})

	t.Run("same attachment named twice is selected once", func(t *testing.T) {
		out, err := selectAttachments(all, []string{"b.txt", "11"})
		assert.NoError(t, err)
		assert.Equal(t, []jira.Attachment{all[1]}, out)
	})

	t.Run("unknown ref", func(t *testing.T) {
		_, err := selectAttachments(all, []string{"11", "nope.txt"})
		assert.ErrorContains(t, err, `"nope.txt"`)
	})
}

func TestTargetNames(t *testing.T) {
	names := targetNames([]jira.Attachment{
		{ID: "10", Filename: "a.txt"},
		{ID: "11", Filename: "b.txt"},
		{ID: "12", Filename: "a.txt"},
		{ID: "13", Filename: "../../etc/passwd"},
		{ID: "14", Filename: ".."},
		{ID: "15", Filename: `..\evil.sh`},
	})

	assert.Equal(t, []string{"a-10.txt", "b.txt", "a-12.txt", "passwd", "attachment-14", "evil.sh"}, names)
}
