package md

import (
	"bytes"

	bf "github.com/russross/blackfriday/v2"
)

// isBareLink reports whether a link node is shaped like an autolink, where the
// visible text is the destination itself. blackfriday gives `<url>` and
// `[url](url)` the same shape, which is fine: neither carries text of its own.
func isBareLink(n *bf.Node) bool {
	child := n.FirstChild
	return len(n.Destination) > 0 &&
		child != nil && child == n.LastChild &&
		child.Type == bf.Text && bytes.Equal(child.Literal, n.Destination)
}

// splitTrailingPunctuation moves punctuation that blackfriday glued onto the
// end of an autolink into a text node behind the link. Its own heuristic
// only drops a single trailing ".", "," or ";" and a bracket it can close
// within the line, so "(see https://x/y):" would otherwise end up in the link.
func splitTrailingPunctuation(ast *bf.Node) {
	var links []*bf.Node
	ast.Walk(func(n *bf.Node, entering bool) bf.WalkStatus {
		if entering && n.Type == bf.Link && isBareLink(n) {
			links = append(links, n)
		}
		return bf.GoToNext
	})

	// Done after the walk because it inserts siblings.
	for _, link := range links {
		keep := trimmedLen(link.Destination)
		if keep == len(link.Destination) {
			continue
		}

		tail := bytes.Clone(link.Destination[keep:])
		link.Destination = link.Destination[:keep]
		link.FirstChild.Literal = link.Destination

		text := bf.NewNode(bf.Text)
		text.Literal = tail
		if link.Next != nil {
			link.Next.InsertBefore(text)
		} else {
			link.Parent.AppendChild(text)
		}
	}
}

// trimmedLen returns the length of url without trailing punctuation. A closing
// bracket only counts as punctuation if the URL doesn't open it itself, so
// that "https://x/Pikachu_(Electric)" keeps its parenthesis.
func trimmedLen(url []byte) int {
	// Never trim into the scheme.
	floor := 1
	if i := bytes.Index(url, []byte("://")); i >= 0 {
		floor = i + len("://") + 1
	}

	end := len(url)
	for end > floor {
		c := url[end-1]
		if bytes.IndexByte([]byte(".,:;!?'\""), c) >= 0 {
			end--
			continue
		}

		var opener byte
		switch c {
		case ')':
			opener = '('
		case ']':
			opener = '['
		case '}':
			opener = '{'
		default:
			return end
		}
		if bytes.Count(url[:end], []byte{c}) <= bytes.Count(url[:end], []byte{opener}) {
			return end
		}
		end--
	}
	return end
}
