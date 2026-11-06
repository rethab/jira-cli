package md

import (
	"bytes"
	"regexp"
	"slices"
	"strconv"
	"strings"

	bf "github.com/russross/blackfriday/v2"

	"github.com/rethab/jira-cli/pkg/adf"
)

// cloudMention matches the mention syntax Cloud users are resolved to.
var cloudMention = regexp.MustCompile(`^\[~accountid:([^\[\]\n]+)\]$`)

// ToADF translates CommonMark to an Atlassian Document Format document, for the
// v3 API. It honours the same options as ToJiraMD.
func ToADF(markdown string, opts ...Option) *adf.ADF {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	// Account IDs may contain characters CommonMark treats as emphasis, so
	// mentions are swapped out before parsing, as ToJiraMD does.
	nonce := placeholderNonce()
	var mentions []string
	protected := mentionPattern.ReplaceAllStringFunc(markdown, func(mention string) string {
		mentions = append(mentions, mention)
		return placeholder(nonce, len(mentions)-1)
	})

	ast := bf.New(bf.WithExtensions(bf.CommonExtensions)).Parse([]byte(protected))
	splitTrailingPunctuation(ast, []byte(protected))

	b := &adfBuilder{
		options:  o,
		mentions: mentions,
		marker:   regexp.MustCompile(regexp.QuoteMeta(nonce) + `(\d+)` + regexp.QuoteMeta(nonce)),
	}

	return &adf.ADF{Version: 1, DocType: "doc", Content: b.blocks(ast)}
}

type adfBuilder struct {
	options
	mentions []string
	marker   *regexp.Regexp
}

func (b *adfBuilder) blocks(parent *bf.Node) []*adf.Node {
	out := []*adf.Node{}
	for n := parent.FirstChild; n != nil; n = n.Next {
		out = append(out, b.block(n)...)
	}
	return out
}

func (b *adfBuilder) block(n *bf.Node) []*adf.Node {
	switch n.Type {
	case bf.Paragraph:
		return []*adf.Node{{NodeType: adf.NodeParagraph, Content: b.inlines(n, nil)}}
	case bf.Heading:
		return []*adf.Node{{
			NodeType:   adf.NodeHeading,
			Attributes: map[string]any{"level": n.Level},
			Content:    b.inlines(n, nil),
		}}
	case bf.List:
		kind := adf.NodeBulletList
		if n.ListFlags&bf.ListTypeOrdered != 0 {
			kind = adf.NodeOrderedList
		}
		return []*adf.Node{{NodeType: kind, Content: b.blocks(n)}}
	case bf.Item:
		return []*adf.Node{{NodeType: adf.ChildNodeListItem, Content: nestable(b.blocks(n))}}
	case bf.BlockQuote:
		return []*adf.Node{{NodeType: adf.NodeBlockquote, Content: nestable(b.blocks(n))}}
	case bf.HorizontalRule:
		return []*adf.Node{{NodeType: adf.NodeRule}}
	case bf.CodeBlock:
		return []*adf.Node{b.codeBlock(n)}
	case bf.HTMLBlock:
		literal := strings.TrimRight(string(n.Literal), "\n")
		return []*adf.Node{{NodeType: adf.NodeParagraph, Content: b.textNodes(literal, nil)}}
	case bf.Table:
		return []*adf.Node{{NodeType: adf.NodeTable, Content: b.tableRows(n)}}
	}
	// Unknown containers still hold content worth keeping.
	return b.blocks(n)
}

// ADF only allows paragraphs, lists and code blocks in list items and
// blockquotes, and Jira rejects the whole document otherwise, so headings
// become paragraphs, nested quotes give up their quote and rules are dropped.
func nestable(nodes []*adf.Node) []*adf.Node {
	out := make([]*adf.Node, 0, len(nodes))
	for _, n := range nodes {
		switch n.NodeType {
		case adf.NodeHeading:
			out = append(out, &adf.Node{NodeType: adf.NodeParagraph, Content: n.Content})
		case adf.NodeBlockquote:
			out = append(out, n.Content...)
		case adf.NodeRule:
		default:
			out = append(out, n)
		}
	}
	return out
}

func (b *adfBuilder) codeBlock(n *bf.Node) *adf.Node {
	node := &adf.Node{NodeType: adf.NodeCodeBlock}
	if info := strings.TrimSpace(b.restore(string(n.Info))); info != "" {
		node.Attributes = map[string]any{"language": info}
	}
	if code := strings.TrimRight(b.restore(string(n.Literal)), "\n"); code != "" {
		node.Content = []*adf.Node{textNode(code, nil)}
	}
	return node
}

// tableRows flattens the head and body, which ADF doesn't distinguish.
func (b *adfBuilder) tableRows(table *bf.Node) []*adf.Node {
	var rows []*adf.Node
	for section := table.FirstChild; section != nil; section = section.Next {
		for row := section.FirstChild; row != nil; row = row.Next {
			r := &adf.Node{NodeType: adf.ChildNodeTableRow}
			for cell := row.FirstChild; cell != nil; cell = cell.Next {
				kind := adf.ChildNodeTableCell
				if cell.IsHeader {
					kind = adf.ChildNodeTableHeader
				}
				// Cells need block content, and an empty one an empty paragraph.
				r.Content = append(r.Content, &adf.Node{
					NodeType: kind,
					Content:  []*adf.Node{{NodeType: adf.NodeParagraph, Content: b.inlines(cell, nil)}},
				})
			}
			rows = append(rows, r)
		}
	}
	return rows
}

// inlines converts the children of parent. Marks are inherited from the
// enclosing emphasis, strong, strike and link nodes.
func (b *adfBuilder) inlines(parent *bf.Node, marks []adf.MarkNode) []*adf.Node {
	var out []*adf.Node
	for n := parent.FirstChild; n != nil; n = n.Next {
		switch n.Type {
		case bf.Text:
			// Adjacent text nodes are joined so that a mention can't be split between them.
			var literal bytes.Buffer
			literal.Write(n.Literal)
			for n.Next != nil && n.Next.Type == bf.Text {
				n = n.Next
				literal.Write(n.Literal)
			}
			out = append(out, b.textNodes(literal.String(), marks)...)
		case bf.Emph:
			out = append(out, b.inlines(n, withMark(marks, adf.MarkEm, nil))...)
		case bf.Strong:
			out = append(out, b.inlines(n, withMark(marks, adf.MarkStrong, nil))...)
		case bf.Del:
			out = append(out, b.inlines(n, withMark(marks, adf.MarkStrike, nil))...)
		case bf.Link:
			out = append(out, b.link(n, marks)...)
		case bf.Image:
			out = append(out, b.image(n, marks)...)
		case bf.Code:
			if len(n.Literal) > 0 {
				out = append(out, textNode(b.restore(string(n.Literal)), codeMarks(marks)))
			}
		case bf.Hardbreak, bf.Softbreak:
			out = append(out, &adf.Node{NodeType: adf.InlineNodeHardBreak})
		case bf.HTMLSpan:
			out = append(out, b.textNodes(string(n.Literal), marks)...)
		default:
			out = append(out, b.inlines(n, marks)...)
		}
	}
	return trimBreaks(out)
}

func (b *adfBuilder) link(n *bf.Node, marks []adf.MarkNode) []*adf.Node {
	dest := b.restore(string(n.Destination))
	if b.issueURL != nil && isBareLink(n) && b.issueURL.MatchString(dest) {
		return []*adf.Node{{
			NodeType:   adf.InlineNodeCard,
			Attributes: map[string]any{"url": dest},
		}}
	}
	if dest == "" {
		return b.inlines(n, marks)
	}
	return b.inlines(n, withMark(marks, adf.MarkLink, map[string]any{"href": dest}))
}

// ADF media has to be uploaded to Jira first, which is more than appending text
// should do, so an image is kept as a link to where it lives.
func (b *adfBuilder) image(n *bf.Node, marks []adf.MarkNode) []*adf.Node {
	var alt strings.Builder
	n.Walk(func(c *bf.Node, entering bool) bf.WalkStatus {
		if entering && c != n && c.Type == bf.Text {
			alt.Write(c.Literal)
		}
		return bf.GoToNext
	})

	dest := b.restore(string(n.Destination))
	label := b.restore(alt.String())
	if label == "" {
		label = dest
	}
	if label == "" {
		return nil
	}
	// Jira rejects a second link mark, so an image inside a link keeps the outer one.
	if dest != "" && !slices.ContainsFunc(marks, func(m adf.MarkNode) bool { return m.MarkType == adf.MarkLink }) {
		marks = withMark(marks, adf.MarkLink, map[string]any{"href": dest})
	}
	return []*adf.Node{textNode(label, marks)}
}

// restore puts mentions back as written where they can't become mention nodes,
// such as code and URLs, so the placeholders never reach Jira.
func (b *adfBuilder) restore(s string) string {
	return b.marker.ReplaceAllStringFunc(s, func(m string) string {
		idx, err := strconv.Atoi(b.marker.FindStringSubmatch(m)[1])
		if err != nil || idx >= len(b.mentions) {
			return m
		}
		return b.mentions[idx]
	})
}

// textNodes splits literal into text, mention and hard break nodes. A newline
// inside a text node is a soft line break in CommonMark, which the wiki path
// renders as a line break, so it is one here too.
func (b *adfBuilder) textNodes(literal string, marks []adf.MarkNode) []*adf.Node {
	var out []*adf.Node
	for i, line := range strings.Split(literal, "\n") {
		if i > 0 {
			out = append(out, &adf.Node{NodeType: adf.InlineNodeHardBreak})
		}
		out = append(out, b.lineNodes(line, marks)...)
	}
	return out
}

func (b *adfBuilder) lineNodes(line string, marks []adf.MarkNode) []*adf.Node {
	var out []*adf.Node
	rest := line
	for _, loc := range b.marker.FindAllStringSubmatchIndex(line, -1) {
		idx, err := strconv.Atoi(line[loc[2]:loc[3]])
		if err != nil || idx >= len(b.mentions) {
			continue
		}
		out = append(out, textNode(line[len(line)-len(rest):loc[0]], marks))
		rest = line[loc[1]:]

		original := b.mentions[idx]
		if m := cloudMention.FindStringSubmatch(original); m != nil {
			out = append(out, &adf.Node{NodeType: adf.InlineNodeMention, Attributes: map[string]any{"id": m[1]}})
		} else {
			out = append(out, textNode(original, marks))
		}
	}
	out = append(out, textNode(rest, marks))

	// Jira rejects empty text nodes.
	return filterEmpty(out)
}

func filterEmpty(nodes []*adf.Node) []*adf.Node {
	out := nodes[:0]
	for _, n := range nodes {
		if n.NodeType == adf.ChildNodeText && n.Text == "" {
			continue
		}
		out = append(out, n)
	}
	return out
}

func trimBreaks(nodes []*adf.Node) []*adf.Node {
	for len(nodes) > 0 && nodes[0].NodeType == adf.InlineNodeHardBreak {
		nodes = nodes[1:]
	}
	for len(nodes) > 0 && nodes[len(nodes)-1].NodeType == adf.InlineNodeHardBreak {
		nodes = nodes[:len(nodes)-1]
	}
	return nodes
}

func textNode(s string, marks []adf.MarkNode) *adf.Node {
	n := &adf.Node{NodeType: adf.ChildNodeText}
	n.Text = s
	if len(marks) > 0 {
		n.Marks = append([]adf.MarkNode(nil), marks...)
	}
	return n
}

func withMark(marks []adf.MarkNode, t adf.NodeType, attrs any) []adf.MarkNode {
	out := make([]adf.MarkNode, 0, len(marks)+1)
	out = append(out, marks...)
	return append(out, adf.MarkNode{MarkType: t, Attributes: attrs})
}

// Jira only accepts the code mark together with a link.
func codeMarks(marks []adf.MarkNode) []adf.MarkNode {
	out := []adf.MarkNode{{MarkType: adf.MarkCode}}
	for _, m := range marks {
		if m.MarkType == adf.MarkLink {
			out = append(out, m)
		}
	}
	return out
}
