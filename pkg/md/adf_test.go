package md

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/adf"
)

func doc(content string) string {
	return `{"version":1,"type":"doc","content":[` + content + `]}`
}

func para(content string) string {
	return `{"type":"paragraph","content":[` + content + `]}`
}

func text(s string, marks ...string) string {
	out := `{"type":"text","text":"` + s + `"`
	if len(marks) > 0 {
		out += `,"marks":[`
		for i, m := range marks {
			if i > 0 {
				out += ","
			}
			out += m
		}
		out += `]`
	}
	return out + `}`
}

func mark(t string) string { return `{"type":"` + t + `"}` }

func link(href string) string {
	return `{"type":"link","attrs":{"href":"` + href + `"}}`
}

func item(content string) string {
	return `{"type":"listItem","content":[` + content + `]}`
}

func TestToADF(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "marks",
			in:   "**b** *i* ~~s~~ `c` [t](https://x)",
			want: doc(para(
				text("b", mark("strong")) + "," + text(" ") + "," +
					text("i", mark("em")) + "," + text(" ") + "," +
					text("s", mark("strike")) + "," + text(" ") + "," +
					text("c", mark("code")) + "," + text(" ") + "," +
					text("t", link("https://x")),
			)),
		},
		{
			name: "code inside strong keeps only the code mark",
			in:   "**`c`**",
			want: doc(para(text("c", mark("code")))),
		},
		{
			name: "code inside a link keeps both marks",
			in:   "[`c`](https://x)",
			want: doc(para(text("c", mark("code"), link("https://x")))),
		},
		{
			name: "nested marks",
			in:   "***both***",
			want: doc(para(text("both", mark("strong"), mark("em")))),
		},
		{
			name: "bullet list",
			in:   "- a\n- b",
			want: doc(`{"type":"bulletList","content":[` +
				item(para(text("a"))) + "," + item(para(text("b"))) + `]}`),
		},
		{
			name: "ordered list",
			in:   "1. a\n2. b",
			want: doc(`{"type":"orderedList","content":[` +
				item(para(text("a"))) + "," + item(para(text("b"))) + `]}`),
		},
		{
			name: "nested list",
			in:   "- a\n  - b",
			want: doc(`{"type":"bulletList","content":[` +
				item(para(text("a"))+`,{"type":"bulletList","content":[`+item(para(text("b")))+`]}`) + `]}`),
		},
		{
			name: "headings",
			in:   "# H\n\n### H",
			want: doc(`{"type":"heading","attrs":{"level":1},"content":[` + text("H") + `]},` +
				`{"type":"heading","attrs":{"level":3},"content":[` + text("H") + `]}`),
		},
		{
			name: "blockquote",
			in:   "> q",
			want: doc(`{"type":"blockquote","content":[` + para(text("q")) + `]}`),
		},
		{
			name: "rule",
			in:   "---",
			want: doc(`{"type":"rule"}`),
		},
		{
			name: "fenced code block with language",
			in:   "```go\nfmt.Println()\n```",
			want: doc(`{"type":"codeBlock","attrs":{"language":"go"},"content":[` + text("fmt.Println()") + `]}`),
		},
		{
			name: "fenced code block without language",
			in:   "```\nplain\n```",
			want: doc(`{"type":"codeBlock","content":[` + text("plain") + `]}`),
		},
		{
			name: "empty fenced code block",
			in:   "```\n```",
			want: doc(`{"type":"codeBlock"}`),
		},
		{
			name: "table",
			in:   "| a | b |\n| --- | --- |\n| 1 | 2 |",
			want: doc(`{"type":"table","content":[` +
				`{"type":"tableRow","content":[` +
				`{"type":"tableHeader","content":[` + para(text("a")) + `]},` +
				`{"type":"tableHeader","content":[` + para(text("b")) + `]}]},` +
				`{"type":"tableRow","content":[` +
				`{"type":"tableCell","content":[` + para(text("1")) + `]},` +
				`{"type":"tableCell","content":[` + para(text("2")) + `]}]}]}`),
		},
		{
			name: "empty table cell gets an empty paragraph",
			in:   "| a | b |\n| --- | --- |\n| 1 |  |",
			want: doc(`{"type":"table","content":[` +
				`{"type":"tableRow","content":[` +
				`{"type":"tableHeader","content":[` + para(text("a")) + `]},` +
				`{"type":"tableHeader","content":[` + para(text("b")) + `]}]},` +
				`{"type":"tableRow","content":[` +
				`{"type":"tableCell","content":[` + para(text("1")) + `]},` +
				`{"type":"tableCell","content":[{"type":"paragraph"}]}]}]}`),
		},
		{
			name: "soft line break becomes a hard break",
			in:   "line one\nline two",
			want: doc(para(text("line one") + `,{"type":"hardBreak"},` + text("line two"))),
		},
		{
			name: "mention between text",
			in:   "ping [~accountid:abc123] now",
			want: doc(para(text("ping ") + `,{"type":"mention","attrs":{"id":"abc123"}},` + text(" now"))),
		},
		{
			name: "mention next to emphasis",
			in:   "**hi** [~accountid:abc123]*!*",
			want: doc(para(text("hi", mark("strong")) + "," + text(" ") +
				`,{"type":"mention","attrs":{"id":"abc123"}},` + text("!", mark("em")))),
		},
		{
			name: "other bracketed tildes stay text",
			in:   "[~name]",
			want: doc(para(text("[~name]"))),
		},
		{
			name: "image becomes a link to its url",
			in:   "![alt](https://x/i.png)",
			want: doc(para(text("alt", link("https://x/i.png")))),
		},
		{
			name: "image without alt text uses the url",
			in:   "![](https://x/i.png)",
			want: doc(para(text("https://x/i.png", link("https://x/i.png")))),
		},
		{
			name: "html is kept as text",
			in:   "<b>x</b>",
			want: doc(para(text("<b>") + "," + text("x") + "," + text("</b>"))),
		},
		{
			name: "bare url is a link",
			in:   "https://example.atlassian.net/browse/ABC-1",
			want: doc(para(text("https://example.atlassian.net/browse/ABC-1",
				link("https://example.atlassian.net/browse/ABC-1")))),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(ToADF(tc.in))
			assert.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestToADFSmartLinks(t *testing.T) {
	const url = "https://example.atlassian.net/browse/ABC-1"
	smart := doc(para(`{"type":"inlineCard","attrs":{"url":"` + url + `"}}`))
	plain := doc(para(text(url, link(url))))

	tests := []struct {
		name string
		in   string
		opts []Option
		want string
	}{
		{name: "bare issue url", in: url, opts: []Option{WithSmartLinks("https://example.atlassian.net")}, want: smart},
		{name: "without the option", in: url, want: plain},
		{
			name: "link with its own text",
			in:   "[text](" + url + ")",
			opts: []Option{WithSmartLinks("https://example.atlassian.net")},
			want: doc(para(text("text", link(url)))),
		},
		{
			name: "other site",
			in:   "https://other.atlassian.net/browse/ABC-1",
			opts: []Option{WithSmartLinks("https://example.atlassian.net")},
			want: doc(para(text("https://other.atlassian.net/browse/ABC-1", link("https://other.atlassian.net/browse/ABC-1")))),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(ToADF(tc.in, tc.opts...))
			assert.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

func TestToADFNeverEmitsEmptyTextOrInvalidNodes(t *testing.T) {
	in := "# Title\n\nSome *emph* and **strong `code`** text.\n\n" +
		"- item with [link](https://x)\n  - nested ~~gone~~\n\n" +
		"| a | b |\n| --- | --- |\n| *x* |  |\n\n" +
		"> quote\n> more\n\n```\ncode\n```\n\nping [~accountid:abc] and https://x/y).\n\n" +
		"hard  \nbreak\n\n![](i.png)\n"

	var walk func(nodes []*adf.Node)
	walk = func(nodes []*adf.Node) {
		for _, n := range nodes {
			if n.NodeType == adf.ChildNodeText {
				assert.NotEmpty(t, n.Text)
				for _, m := range n.Marks {
					if m.MarkType == adf.MarkCode {
						for _, other := range n.Marks {
							assert.NotContains(t, []adf.NodeType{adf.MarkEm, adf.MarkStrong, adf.MarkStrike}, other.MarkType)
						}
					}
				}
			}
			walk(n.Content)
		}
	}

	d := ToADF(in)
	assert.Equal(t, 1, d.Version)
	assert.Equal(t, "doc", d.DocType)
	assert.NotEmpty(t, d.Content)
	walk(d.Content)
}

func TestToADFEmptyInput(t *testing.T) {
	got, err := json.Marshal(ToADF(""))
	assert.NoError(t, err)
	assert.JSONEq(t, `{"version":1,"type":"doc","content":[]}`, string(got))
}
