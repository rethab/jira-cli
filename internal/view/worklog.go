package view

import (
	"fmt"
	"strings"

	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/tui"
)

const (
	dateLen          = len("2006-01-02")
	secondsPerHour   = 3600
	secondsPerMinute = 60
)

// IssueWorklogs is a view of a single issue's worklogs.
type IssueWorklogs struct {
	Data []jira.Worklog
	listView
}

// NewIssueWorklogs constructs a view for a single issue's worklogs.
func NewIssueWorklogs(data []jira.Worklog, display ListDisplayFormat, opts ...ListOption) *IssueWorklogs {
	return &IssueWorklogs{Data: data, listView: newListView(display, opts)}
}

// Render renders the view.
func (w *IssueWorklogs) Render() error {
	return w.render(w.data())
}

func (w *IssueWorklogs) data() tui.TableData {
	data := make(tui.TableData, 1, 1+len(w.Data))
	data[0] = []string{"ID", "DATE", "TIME SPENT", "AUTHOR", "COMMENT"}
	for _, wl := range w.Data {
		data = append(data, []string{wl.ID, dateOnly(wl.Started), wl.TimeSpent, wl.Author.Name, oneLine(wl.Comment)})
	}
	return data
}

// WorklogReportEntry is a single row of a cross-issue worklog report.
type WorklogReportEntry struct {
	IssueKey         string
	ID               string
	IssueSummary     string
	TimeSpent        string
	TimeSpentSeconds int64
	Comment          string
}

// WorklogReport is a view of worklogs a user logged across issues on a given day.
type WorklogReport struct {
	Data []WorklogReportEntry
	listView
}

// NewWorklogReport constructs a view for a cross-issue worklog report.
func NewWorklogReport(data []WorklogReportEntry, display ListDisplayFormat, opts ...ListOption) *WorklogReport {
	return &WorklogReport{Data: data, listView: newListView(display, opts)}
}

// Render renders the view. The CSV format contains only the data rows, so
// the total row does not throw off consumers that sum the column themselves.
func (w *WorklogReport) Render() error {
	data := w.data()
	if !w.display.CSV {
		data = append(data, []string{"TOTAL", "", "", formatDuration(w.total()), ""})
	}
	return w.render(data)
}

func (w *WorklogReport) data() tui.TableData {
	data := make(tui.TableData, 1, 1+len(w.Data))
	data[0] = []string{"ISSUE", "ID", "SUMMARY", "TIME SPENT", "COMMENT"}
	for _, e := range w.Data {
		data = append(data, []string{e.IssueKey, e.ID, oneLine(e.IssueSummary), e.TimeSpent, oneLine(e.Comment)})
	}
	return data
}

func (w *WorklogReport) total() int64 {
	var total int64
	for _, e := range w.Data {
		total += e.TimeSpentSeconds
	}
	return total
}

func formatDuration(seconds int64) string {
	return fmt.Sprintf("%dh %02dm", seconds/secondsPerHour, (seconds%secondsPerHour)/secondsPerMinute)
}

func dateOnly(started string) string {
	if len(started) < dateLen {
		return started
	}
	return started[:dateLen]
}

// oneLine collapses a worklog comment to a single line so it doesn't break
// table/CSV rows.
func oneLine(comment string) string {
	return strings.Join(strings.Fields(comment), " ")
}
