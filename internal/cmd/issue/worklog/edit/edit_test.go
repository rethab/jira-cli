package edit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func ptr(s string) *string { return &s }

func TestNothingToUpdate(t *testing.T) {
	tests := []struct {
		name     string
		params   editParams
		expected bool
	}{
		{name: "no changes", params: editParams{}, expected: true},
		{name: "time spent", params: editParams{timeSpent: "1h"}},
		{name: "started", params: editParams{started: "2022-01-01T09:30:00.000+0000"}},
		{name: "comment", params: editParams{comment: ptr("new")}},
		{name: "empty comment clears it", params: editParams{comment: ptr("")}},
		{name: "new estimate", params: editParams{newEstimate: "0h"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.params.nothingToUpdate())
		})
	}
}

func TestMerge(t *testing.T) {
	existing := &jira.Worklog{
		ID:         "10042",
		Comment:    "old *comment*",
		Started:    "2022-01-01T09:30:00.000+0000",
		TimeSpent:  "1h",
		Visibility: &jira.CommentVisibility{Type: "role", Value: "Developers"},
	}

	t.Run("unspecified fields are kept as they are", func(t *testing.T) {
		p := editParams{timeSpent: "2h"}

		assert.Equal(t, jira.WorklogUpdate{
			Started:    "2022-01-01T09:30:00.000+0000",
			TimeSpent:  "2h",
			Comment:    "old *comment*",
			Visibility: existing.Visibility,
		}, p.merge(existing))
	})

	t.Run("comment is converted to wiki markup", func(t *testing.T) {
		p := editParams{comment: ptr("new **comment**")}

		got := p.merge(existing)
		assert.Equal(t, "new *comment*", got.Comment)
		assert.Equal(t, "1h", got.TimeSpent)
	})

	t.Run("empty comment clears it", func(t *testing.T) {
		p := editParams{comment: ptr("")}

		assert.Equal(t, "", p.merge(existing).Comment)
	})

	t.Run("started is replaced", func(t *testing.T) {
		p := editParams{started: "2022-02-02T10:00:00.000+0100"}

		got := p.merge(existing)
		assert.Equal(t, "2022-02-02T10:00:00.000+0100", got.Started)
		assert.Equal(t, "old *comment*", got.Comment)
	})
}
