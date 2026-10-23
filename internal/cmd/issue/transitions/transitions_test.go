package transitions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func newTransition(name, to string, available bool) *jira.Transition {
	t := &jira.Transition{Name: name, IsAvailable: available}
	t.To.Name = to
	return t
}

func TestFilterAvailable(t *testing.T) {
	all := []*jira.Transition{
		newTransition("Start work", "In Progress", true),
		newTransition("Close", "Done", false),
	}

	t.Run("cloud hides transitions that are not available", func(t *testing.T) {
		got := filterAvailable(all, jira.InstallationTypeCloud)

		assert.Equal(t, []*jira.Transition{all[0]}, got)
	})

	t.Run("local keeps all transitions since isAvailable is not returned", func(t *testing.T) {
		got := filterAvailable(all, jira.InstallationTypeLocal)

		assert.Equal(t, all, got)
	})
}

func TestFormat(t *testing.T) {
	transitions := []*jira.Transition{
		newTransition("Start work", "In Progress", true),
		newTransition("Done", "Done", true),
	}

	assert.Equal(t, "Start work -> In Progress\nDone -> Done", format(transitions, false))
	assert.Equal(t, "Start work\tIn Progress\nDone\tDone", format(transitions, true))
	assert.Empty(t, format(nil, false))
}
