package view

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/tui"
)

const (
	dateLen          = len("2006-01-02")
	secondsPerHour   = 3600
	secondsPerMinute = 60
)

// WorklogDisplayFormat controls how a worklog view is rendered.
type WorklogDisplayFormat struct {
	Plain bool
	CSV   bool
}

// IssueWorklogsOption is a functional option to configure IssueWorklogs.
type IssueWorklogsOption func(*IssueWorklogs)

// IssueWorklogs is a view of a single issue's worklogs.
type IssueWorklogs struct {
	Data    []jira.Worklog
	Display WorklogDisplayFormat
	writer  io.Writer
	buf     *bytes.Buffer
}

// NewIssueWorklogs constructs a view for a single issue's worklogs.
func NewIssueWorklogs(data []jira.Worklog, display WorklogDisplayFormat, opts ...IssueWorklogsOption) *IssueWorklogs {
	w := IssueWorklogs{
		Data:    data,
		Display: display,
		buf:     new(bytes.Buffer),
	}
	w.writer = tabwriter.NewWriter(w.buf, 0, tabWidth, 1, '\t', 0)

	for _, opt := range opts {
		opt(&w)
	}
	return &w
}

// WithIssueWorklogsWriter sets the writer used for the pager-backed render.
func WithIssueWorklogsWriter(out io.Writer) IssueWorklogsOption {
	return func(w *IssueWorklogs) {
		w.writer = out
	}
}

// Render renders the view.
func (w *IssueWorklogs) Render() error {
	data := w.data()

	if w.Display.CSV {
		return renderCSV(os.Stdout, data)
	}
	if w.Display.Plain {
		return renderPlain(tabwriter.NewWriter(os.Stdout, 0, tabWidth, 1, '\t', 0), data, "\t")
	}
	if err := renderPlain(w.writer, data, "\t"); err != nil {
		return err
	}
	return tui.PagerOut(w.buf.String())
}

func (w *IssueWorklogs) data() tui.TableData {
	data := make(tui.TableData, 1, 1+len(w.Data))
	data[0] = []string{"DATE", "TIME SPENT", "AUTHOR", "COMMENT"}
	for _, wl := range w.Data {
		data = append(data, []string{dateOnly(wl.Started), wl.TimeSpent, wl.Author.Name, oneLine(wl.Comment)})
	}
	return data
}

// WorklogReportEntry is a single row of a cross-issue worklog report.
type WorklogReportEntry struct {
	IssueKey         string
	TimeSpent        string
	TimeSpentSeconds int64
	Comment          string
}

// WorklogReportOption is a functional option to configure WorklogReport.
type WorklogReportOption func(*WorklogReport)

// WorklogReport is a view of worklogs a user logged across issues on a given day.
type WorklogReport struct {
	Data    []WorklogReportEntry
	Display WorklogDisplayFormat
	writer  io.Writer
	buf     *bytes.Buffer
}

// NewWorklogReport constructs a view for a cross-issue worklog report.
func NewWorklogReport(data []WorklogReportEntry, display WorklogDisplayFormat, opts ...WorklogReportOption) *WorklogReport {
	w := WorklogReport{
		Data:    data,
		Display: display,
		buf:     new(bytes.Buffer),
	}
	w.writer = tabwriter.NewWriter(w.buf, 0, tabWidth, 1, '\t', 0)

	for _, opt := range opts {
		opt(&w)
	}
	return &w
}

// WithWorklogReportWriter sets the writer used for the pager-backed render.
func WithWorklogReportWriter(out io.Writer) WorklogReportOption {
	return func(w *WorklogReport) {
		w.writer = out
	}
}

// Render renders the view. The CSV format contains only the data rows, so
// the total row does not throw off consumers that sum the column themselves.
func (w *WorklogReport) Render() error {
	if w.Display.CSV {
		return renderCSV(os.Stdout, w.data())
	}

	data := w.data()
	data = append(data, []string{"TOTAL", formatDuration(w.total()), ""})

	if w.Display.Plain {
		return renderPlain(tabwriter.NewWriter(os.Stdout, 0, tabWidth, 1, '\t', 0), data, "\t")
	}
	if err := renderPlain(w.writer, data, "\t"); err != nil {
		return err
	}
	return tui.PagerOut(w.buf.String())
}

func (w *WorklogReport) data() tui.TableData {
	data := make(tui.TableData, 1, 1+len(w.Data))
	data[0] = []string{"ISSUE", "TIME SPENT", "COMMENT"}
	for _, e := range w.Data {
		data = append(data, []string{e.IssueKey, e.TimeSpent, oneLine(e.Comment)})
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
