package view

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func TestIssueWorklogsRender(t *testing.T) {
	var b bytes.Buffer

	data := []jira.Worklog{
		{
			Started:   "2026-01-14T09:00:00.000-0500",
			TimeSpent: "15m",
			Author:    jira.WorklogAuthor{Name: "Person A"},
			Comment:   "standup",
		},
		{
			Started:   "2026-01-15T09:00:00.000-0500",
			TimeSpent: "1h",
			Author:    jira.WorklogAuthor{Name: "Person B"},
			Comment:   "sprint\nplanning",
		},
	}

	w := NewIssueWorklogs(data, WorklogDisplayFormat{}, WithIssueWorklogsWriter(&b))
	assert.NoError(t, w.Render())

	expected := `DATE	TIME SPENT	AUTHOR	COMMENT
2026-01-14	15m	Person A	standup
2026-01-15	1h	Person B	sprint planning
`
	assert.Equal(t, expected, b.String())
}

func TestWorklogReportRender(t *testing.T) {
	var b bytes.Buffer

	data := []WorklogReportEntry{
		{IssueKey: "TEST-1", IssueSummary: "Fix the thing", TimeSpent: "15m", TimeSpentSeconds: 900, Comment: "standup"},
		{IssueKey: "TEST-2", IssueSummary: "Ship the other thing", TimeSpent: "1h 15m", TimeSpentSeconds: 4500, Comment: "sprint planning"},
	}

	w := NewWorklogReport(data, WorklogDisplayFormat{}, WithWorklogReportWriter(&b))
	assert.NoError(t, w.Render())

	expected := "ISSUE\tSUMMARY\tTIME SPENT\tCOMMENT\n" +
		"TEST-1\tFix the thing\t15m\tstandup\n" +
		"TEST-2\tShip the other thing\t1h 15m\tsprint planning\n" +
		"TOTAL\t\t1h 30m\t\n"
	assert.Equal(t, expected, b.String())
}

func TestWorklogReportRenderCSVExcludesTotal(t *testing.T) {
	data := []WorklogReportEntry{
		{IssueKey: "TEST-1", TimeSpent: "15m", TimeSpentSeconds: 900, Comment: "standup"},
	}

	w := NewWorklogReport(data, WorklogDisplayFormat{CSV: true})
	assert.NoError(t, w.Render())
}
