package view

import (
	"fmt"

	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/tui"
)

const bytesPerUnit = 1024

// IssueAttachments is a view of a single issue's attachments.
type IssueAttachments struct {
	Data []jira.Attachment
	listView
}

// NewIssueAttachments constructs a view for a single issue's attachments.
func NewIssueAttachments(data []jira.Attachment, display ListDisplayFormat, opts ...ListOption) *IssueAttachments {
	return &IssueAttachments{Data: data, listView: newListView(display, opts)}
}

// Render renders the view.
func (a *IssueAttachments) Render() error {
	return a.render(a.data())
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
