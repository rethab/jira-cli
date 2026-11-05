package changelog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func sample() []jira.ChangelogHistory {
	return []jira.ChangelogHistory{
		{
			ID:      "1",
			Author:  jira.User{DisplayName: "Jane Doe", Name: "jdoe"},
			Created: "2026-10-01T14:03:00.000+0000",
			Items: []jira.ChangelogItem{
				{Field: "status", FieldID: "status", From: "1", FromString: "In Testing", To: "2", ToString: "To Do"},
				{Field: "Fix Version", FieldID: "fixVersions", ToString: "1.0"},
			},
		},
		{
			ID:      "2",
			Created: "not a date",
			Items:   []jira.ChangelogItem{{Field: "assignee", From: "abc", To: ""}},
		},
	}
}

func TestFilter(t *testing.T) {
	t.Run("empty field keeps everything", func(t *testing.T) {
		assert.Equal(t, sample(), filter(sample(), ""))
	})

	t.Run("matches field name case-insensitively and drops other items", func(t *testing.T) {
		got := filter(sample(), "STATUS")

		assert.Len(t, got, 1)
		assert.Equal(t, "1", got[0].ID)
		assert.Len(t, got[0].Items, 1)
		assert.Equal(t, "status", got[0].Items[0].Field)
	})

	t.Run("matches field id", func(t *testing.T) {
		got := filter(sample(), "fixversions")

		assert.Len(t, got, 1)
		assert.Equal(t, "Fix Version", got[0].Items[0].Field)
	})

	t.Run("no match", func(t *testing.T) {
		assert.Empty(t, filter(sample(), "labels"))
	})
}

func TestFormat(t *testing.T) {
	original := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = original })

	t.Run("pretty aligns columns and fills gaps", func(t *testing.T) {
		want := "2026-10-01 14:03  Jane Doe    status       In Testing -> To Do\n" +
			"2026-10-01 14:03  Jane Doe    Fix Version  - -> 1.0\n" +
			"not a date        Automation  assignee     abc -> -"

		assert.Equal(t, want, format(sample(), false))
	})

	t.Run("plain is tab separated with raw values", func(t *testing.T) {
		want := "2026-10-01T14:03:00.000+0000\tJane Doe\tstatus\tIn Testing\tTo Do\n" +
			"2026-10-01T14:03:00.000+0000\tJane Doe\tFix Version\t\t1.0\n" +
			"not a date\tAutomation\tassignee\tabc\t"

		assert.Equal(t, want, format(sample(), true))
	})

	t.Run("multi-line values stay on one line", func(t *testing.T) {
		histories := []jira.ChangelogHistory{{
			Created: "x",
			Items:   []jira.ChangelogItem{{Field: "description", FromString: "a\r\nb", ToString: "c\td\ne"}},
		}}

		assert.Equal(t, "x  Automation  description  a b -> c d e", format(histories, false))
		assert.Equal(t, "x\tAutomation\tdescription\ta b\tc d e", format(histories, true))
	})

	t.Run("no histories", func(t *testing.T) {
		assert.Empty(t, format(nil, false))
		assert.Empty(t, format(nil, true))
	})
}

func TestAuthor(t *testing.T) {
	assert.Equal(t, "Jane Doe", author(jira.User{DisplayName: "Jane Doe", Name: "jdoe"}))
	assert.Equal(t, "jdoe", author(jira.User{Name: "jdoe"}))
	assert.Equal(t, "Automation", author(jira.User{}))
}
