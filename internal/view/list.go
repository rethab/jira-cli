package view

import (
	"bytes"
	"io"
	"os"
	"text/tabwriter"

	"github.com/rethab/jira-cli/pkg/tui"
)

// ListDisplayFormat controls how the comment, attachment and worklog lists are rendered.
type ListDisplayFormat struct {
	Plain     bool
	CSV       bool
	NoHeaders bool
}

// ListOption is a functional option to configure the comment, attachment and worklog lists.
type ListOption func(*listView)

// WithListWriter sets the writer used for the pager-backed render.
func WithListWriter(out io.Writer) ListOption {
	return func(l *listView) {
		l.writer = out
	}
}

type listView struct {
	display ListDisplayFormat
	writer  io.Writer
	buf     *bytes.Buffer
}

func newListView(display ListDisplayFormat, opts []ListOption) listView {
	l := listView{display: display, buf: new(bytes.Buffer)}
	l.writer = tabwriter.NewWriter(l.buf, 0, tabWidth, 1, '\t', 0)

	for _, opt := range opts {
		opt(&l)
	}
	return l
}

func (l listView) render(data tui.TableData) error {
	if l.display.CSV {
		return renderCSV(os.Stdout, data)
	}
	if l.display.Plain {
		if l.display.NoHeaders {
			data = data[1:]
		}
		return renderPlain(tabwriter.NewWriter(os.Stdout, 0, tabWidth, 1, '\t', 0), data, "\t")
	}
	if err := renderPlain(l.writer, data, "\t"); err != nil {
		return err
	}
	return tui.PagerOut(l.buf.String())
}
