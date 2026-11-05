package transitions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func newTransition(name, to string) *jira.Transition {
	t := &jira.Transition{Name: name, IsAvailable: true}
	t.To.Name = to
	return t
}

func TestFormat(t *testing.T) {
	transitions := []*jira.Transition{
		newTransition("Start work", "In Progress"),
		newTransition("Done", "Done"),
	}

	assert.Equal(t, "Start work -> In Progress\nDone -> Done", format(transitions, false))
	assert.Equal(t, "Start work\tIn Progress\nDone\tDone", format(transitions, true))
	assert.Empty(t, format(nil, false))

	noTarget := []*jira.Transition{newTransition("Reopen", "")}
	assert.Equal(t, "Reopen", format(noTarget, false))
	assert.Equal(t, "Reopen\t", format(noTarget, true))
}

func TestFormatWithFields(t *testing.T) {
	start := newTransition("Start work", "In Progress")

	rejected := newTransition("Rejected", "Rejected")
	rejected.HasScreen = true
	rejected.Fields = map[string]jira.TransitionField{
		"worklog": {Name: "Log Work"},
		"resolution": {
			Name:     "Resolution",
			Required: true,
			AllowedValues: []jira.TransitionFieldValue{
				{ID: "1", Name: "Done"}, {ID: "2", Name: "Won't Do"}, {ID: "3", Value: "Duplicate"},
			},
		},
	}

	all := []*jira.Transition{start, rejected}

	t.Run("pretty", func(t *testing.T) {
		want := "Start work -> In Progress\n" +
			"Rejected -> Rejected (screen)\n" +
			"    resolution  Resolution  required  Done, Won't Do, Duplicate\n" +
			"    worklog     Log Work    optional"

		assert.Equal(t, want, formatWithFields(all, false))
	})

	t.Run("plain has a line per field and one for transitions without fields", func(t *testing.T) {
		want := "Start work\tIn Progress\t\t\t\t\n" +
			"Rejected\tRejected\tresolution\tResolution\trequired\tDone,Won't Do,Duplicate\n" +
			"Rejected\tRejected\tworklog\tLog Work\toptional\t"

		assert.Equal(t, want, formatWithFields(all, true))
	})

	t.Run("pretty without target status", func(t *testing.T) {
		assert.Equal(t, "Reopen", formatWithFields([]*jira.Transition{newTransition("Reopen", "")}, false))
	})

	assert.Empty(t, formatWithFields(nil, false))
	assert.Empty(t, formatWithFields(nil, true))
}
