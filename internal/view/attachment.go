package view

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/tui"
)

const bytesPerUnit = 1024

// AttachmentDisplayFormat controls how an attachment view is rendered.
type AttachmentDisplayFormat struct {
	Plain bool
	CSV   bool
}

// IssueAttachmentsOption is a functional option to configure IssueAttachments.
type IssueAttachmentsOption func(*IssueAttachments)

// IssueAttachments is a view of a single issue's attachments.
type IssueAttachments struct {
	Data    []jira.Attachment
	Display AttachmentDisplayFormat
	writer  io.Writer
	buf     *bytes.Buffer
}

// NewIssueAttachments constructs a view for a single issue's attachments.
func NewIssueAttachments(data []jira.Attachment, display AttachmentDisplayFormat, opts ...IssueAttachmentsOption) *IssueAttachments {
	a := IssueAttachments{
		Data:    data,
		Display: display,
		buf:     new(bytes.Buffer),
	}
	a.writer = tabwriter.NewWriter(a.buf, 0, tabWidth, 1, '\t', 0)

	for _, opt := range opts {
		opt(&a)
	}
	return &a
}

// WithIssueAttachmentsWriter sets the writer used for the pager-backed render.
func WithIssueAttachmentsWriter(out io.Writer) IssueAttachmentsOption {
	return func(a *IssueAttachments) {
		a.writer = out
	}
}

// Render renders the view.
func (a *IssueAttachments) Render() error {
	data := a.data()

	if a.Display.CSV {
		return renderCSV(os.Stdout, data)
	}
	if a.Display.Plain {
		return renderPlain(tabwriter.NewWriter(os.Stdout, 0, tabWidth, 1, '\t', 0), data, "\t")
	}
	if err := renderPlain(a.writer, data, "\t"); err != nil {
		return err
	}
	return tui.PagerOut(a.buf.String())
}

func (a *IssueAttachments) data() tui.TableData {
	data := make(tui.TableData, 1, 1+len(a.Data))
	data[0] = []string{"ID", "FILENAME", "SIZE", "AUTHOR", "CREATED"}
	for _, at := range a.Data {
		data = append(data, []string{
			at.ID,
			at.Filename,
			formatSize(at.Size),
			userName(at.Author),
			formatDateTime(at.Created, jira.RFC3339, ""),
		})
	}
	return data
}

func formatSize(size int64) string {
	if size < bytesPerUnit {
		return fmt.Sprintf("%d B", size)
	}

	value := float64(size)
	unit := "KB"
	for _, next := range []string{"MB", "GB"} {
		if value/bytesPerUnit < bytesPerUnit {
			break
		}
		value /= bytesPerUnit
		unit = next
	}
	return fmt.Sprintf("%.1f %s", value/bytesPerUnit, unit)
}
