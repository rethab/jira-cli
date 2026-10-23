package view

import (
	"bytes"
	"io"
	"os"
	"text/tabwriter"

	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/md"
	"github.com/rethab/jira-cli/pkg/tui"
)

// CommentDisplayFormat controls how a comment view is rendered.
type CommentDisplayFormat struct {
	Plain     bool
	CSV       bool
	NoHeaders bool
}

// IssueCommentsOption is a functional option to configure IssueComments.
type IssueCommentsOption func(*IssueComments)

// IssueComments is a view of a single issue's comments.
type IssueComments struct {
	Data    []jira.Comment
	Display CommentDisplayFormat
	writer  io.Writer
	buf     *bytes.Buffer
}

// NewIssueComments constructs a view for a single issue's comments.
func NewIssueComments(data []jira.Comment, display CommentDisplayFormat, opts ...IssueCommentsOption) *IssueComments {
	c := IssueComments{
		Data:    data,
		Display: display,
		buf:     new(bytes.Buffer),
	}
	c.writer = tabwriter.NewWriter(c.buf, 0, tabWidth, 1, '\t', 0)

	for _, opt := range opts {
		opt(&c)
	}
	return &c
}

// WithIssueCommentsWriter sets the writer used for the pager-backed render.
func WithIssueCommentsWriter(out io.Writer) IssueCommentsOption {
	return func(c *IssueComments) {
		c.writer = out
	}
}

// Render renders the view.
func (c *IssueComments) Render() error {
	data := c.data()

	if c.Display.CSV {
		return renderCSV(os.Stdout, data)
	}
	if c.Display.Plain {
		if c.Display.NoHeaders {
			data = data[1:]
		}
		return renderPlain(tabwriter.NewWriter(os.Stdout, 0, tabWidth, 1, '\t', 0), data, "\t")
	}
	if err := renderPlain(c.writer, data, "\t"); err != nil {
		return err
	}
	return tui.PagerOut(c.buf.String())
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
			oneLine(md.FromJiraMD(cm.Body)),
		})
	}
	return data
}

func userName(u jira.User) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Name
}
