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
			ID:        "10001",
			Started:   "2026-01-14T09:00:00.000-0500",
			TimeSpent: "15m",
			Author:    jira.WorklogAuthor{Name: "Person A"},
			Comment:   "standup",
		},
		{
			ID:        "10002",
			Started:   "2026-01-15T09:00:00.000-0500",
			TimeSpent: "1h",
			Author:    jira.WorklogAuthor{Name: "Person B"},
			Comment:   "sprint\nplanning",
		},
	}

	w := NewIssueWorklogs(data, ListDisplayFormat{}, WithListWriter(&b))
	assert.NoError(t, w.Render())

	expected := `ID	DATE	TIME SPENT	AUTHOR	COMMENT
10001	2026-01-14	15m	Person A	standup
10002	2026-01-15	1h	Person B	sprint planning
`
	assert.Equal(t, expected, b.String())
}

func TestWorklogReportRender(t *testing.T) {
	var b bytes.Buffer

	data := []WorklogReportEntry{
		{IssueKey: "TEST-1", ID: "10001", IssueSummary: "Fix the thing", TimeSpent: "15m", TimeSpentSeconds: 900, Comment: "standup"},
		{IssueKey: "TEST-2", ID: "10002", IssueSummary: "Ship the other thing", TimeSpent: "1h 15m", TimeSpentSeconds: 4500, Comment: "sprint planning"},
	}

	w := NewWorklogReport(data, ListDisplayFormat{}, WithListWriter(&b))
	assert.NoError(t, w.Render())

	expected := "ISSUE\tID\tSUMMARY\tTIME SPENT\tCOMMENT\n" +
		"TEST-1\t10001\tFix the thing\t15m\tstandup\n" +
		"TEST-2\t10002\tShip the other thing\t1h 15m\tsprint planning\n" +
		"TOTAL\t\t\t1h 30m\t\n"
	assert.Equal(t, expected, b.String())
}

func TestWorklogReportRenderCSVExcludesTotal(t *testing.T) {
	data := []WorklogReportEntry{
		{IssueKey: "TEST-1", TimeSpent: "15m", TimeSpentSeconds: 900, Comment: "standup"},
	}

	w := NewWorklogReport(data, ListDisplayFormat{CSV: true})
	assert.NoError(t, w.Render())
}
