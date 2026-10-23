package view

import (
	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/md"
	"github.com/rethab/jira-cli/pkg/tui"
)

// IssueComments is a view of a single issue's comments.
type IssueComments struct {
	Data []jira.Comment
	listView
}

// NewIssueComments constructs a view for a single issue's comments.
func NewIssueComments(data []jira.Comment, display ListDisplayFormat, opts ...ListOption) *IssueComments {
	return &IssueComments{Data: data, listView: newListView(display, opts)}
}

// Render renders the view.
func (c *IssueComments) Render() error {
	return c.render(c.data())
}

func (c *IssueComments) data() tui.TableData {
	data := make(tui.TableData, 1, 1+len(c.Data))
	data[0] = []string{"ID", "AUTHOR", "CREATED", "UPDATED", "BODY"}
	for _, cm := range c.Data {
		data = append(data, []string{
			cm.ID,
			userName(cm.Author),
			formatDateTime(cm.Created, jira.RFC3339, ""),
			formatDateTime(cm.Updated, jira.RFC3339, ""),
			oneLine(commentBody(cm.Body)),
		})
	}
	return data
}

// commentBody renders a comment body like the issue view does, except that it
// always sticks to ASCII markup: the table is aligned by counting runes, so a
// double-width emoji such as the one for smart links would shift the columns.
func commentBody(body any) string {
	if doc, ok := body.(*adf.ADF); ok {
		return adf.NewTranslator(doc, adf.NewMarkdownTranslator(adf.WithMarkdownASCII())).Translate()
	}
	s, _ := body.(string)
	return md.FromJiraMD(s)
}

func userName(u jira.User) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Name
}
