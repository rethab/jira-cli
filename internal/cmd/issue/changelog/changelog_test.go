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

func descriptionChange(from, to string) []jira.ChangelogHistory {
	return []jira.ChangelogHistory{{
		Author:  jira.User{DisplayName: "Jane Doe"},
		Created: "2026-10-01T14:03:00.000+0000",
		Items:   []jira.ChangelogItem{{Field: "description", FromString: from, ToString: to}},
	}}
}

func TestFormatDiff(t *testing.T) {
	original := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = original })

	const header = "2026-10-01 14:03  Jane Doe  description\n"
	const from = "Steps:\nopen the page\nclick save"
	const to = "Steps:\nopen the page\nclick save twice"
	const wantMultiLine = header + "--- from\n+++ to\n@@ -1,3 +1,3 @@\n Steps:\n open the page\n-click save\n+click save twice"

	t.Run("multi-line change keeps line breaks", func(t *testing.T) {
		assert.Equal(t, wantMultiLine, formatDiff(descriptionChange(from, to)))
	})

	t.Run("crlf is normalised", func(t *testing.T) {
		crlf := "Steps:\r\nopen the page\r\nclick save"

		assert.Equal(t, wantMultiLine, formatDiff(descriptionChange(crlf, to)))
	})

	t.Run("no missing newline marker", func(t *testing.T) {
		assert.NotContains(t, formatDiff(descriptionChange(from, to)), "No newline")
	})

	t.Run("field set for the first time", func(t *testing.T) {
		want := header + "--- from\n+++ to\n@@ -0,0 +1,2 @@\n+first line\n+second line"

		assert.Equal(t, want, formatDiff(descriptionChange("", "first line\nsecond line")))
	})

	t.Run("field cleared", func(t *testing.T) {
		want := header + "--- from\n+++ to\n@@ -1,2 +0,0 @@\n-a\n-b"

		assert.Equal(t, want, formatDiff(descriptionChange("a\nb", "")))
	})

	t.Run("one-line fields and fallback to ids", func(t *testing.T) {
		want := "2026-10-01 14:03  Jane Doe  status\n--- from\n+++ to\n@@ -1 +1 @@\n-In Testing\n+To Do\n\n" +
			"2026-10-01 14:03  Jane Doe  Fix Version\n--- from\n+++ to\n@@ -0,0 +1 @@\n+1.0\n\n" +
			"not a date  Automation  assignee\n--- from\n+++ to\n@@ -1 +0,0 @@\n-abc"

		assert.Equal(t, want, formatDiff(sample()))
	})

	t.Run("identical values", func(t *testing.T) {
		assert.Equal(t, header+"(values are identical)", formatDiff(descriptionChange("same\ntext", "same\ntext")))
	})

	t.Run("only line endings differ", func(t *testing.T) {
		assert.Equal(t, header+"(only line endings or a trailing line break differ)",
			formatDiff(descriptionChange("a\r\nb", "a\nb")))
	})

	t.Run("only a trailing line break differs", func(t *testing.T) {
		assert.Equal(t, header+"(only line endings or a trailing line break differ)",
			formatDiff(descriptionChange("a\nb", "a\nb\n")))
	})

	t.Run("no histories", func(t *testing.T) {
		assert.Empty(t, formatDiff(nil))
	})
}

func TestDiffText(t *testing.T) {
	for in, want := range map[string]string{
		"":       "",
		"a":      "a\n",
		"a\n":    "a\n",
		"a\r\nb": "a\nb\n",
		"a\rb":   "a\nb\n",
	} {
		assert.Equal(t, want, diffText(in), "input %q", in)
	}
}
