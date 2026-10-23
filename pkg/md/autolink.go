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
// That bracket check also counts openers inside the URL, so it cuts the ")"
// off "https://x/Pikachu_(Electric)"; such a bracket is moved back in first.
//
// source is the markdown the AST was parsed from. A `<url>` or `[url](url)`
// spells out where the URL ends, so it is left alone, but the AST can't tell
// those apart from a bare URL; finding the delimited form in the source can.
func splitTrailingPunctuation(ast *bf.Node, source []byte) {
	var links []*bf.Node
	ast.Walk(func(n *bf.Node, entering bool) bf.WalkStatus {
		if entering && n.Type == bf.Link && isBareLink(n) && !isDelimited(n.Destination, source) {
			links = append(links, n)
		}
		return bf.GoToNext
	})

	// Done after the walk because it inserts siblings.
	for _, link := range links {
		reclaimClosingBrackets(link)

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

func isDelimited(url, source []byte) bool {
	return bytes.Contains(source, append(append([]byte("<"), url...), '>')) ||
		bytes.Contains(source, append(append([]byte("]("), url...), ')'))
}

// reclaimClosingBrackets appends closing brackets from the text right after the
// link to it, as long as each one closes a bracket the URL opened.
func reclaimClosingBrackets(link *bf.Node) {
	next := link.Next
	if next == nil || next.Type != bf.Text {
		return
	}

	n := 0
	dest := bytes.Clone(link.Destination)
	for n < len(next.Literal) {
		c := next.Literal[n]
		opener, ok := bracketOpener(c)
		if !ok || bytes.Count(dest, []byte{c}) >= bytes.Count(dest, []byte{opener}) {
			break
		}
		dest = append(dest, c)
		n++
	}
	if n == 0 {
		return
	}

	link.Destination = dest
	link.FirstChild.Literal = dest
	next.Literal = next.Literal[n:]
	if len(next.Literal) == 0 {
		next.Unlink()
	}
}

// trimmedLen returns the length of url without trailing punctuation. A closing
// bracket or quote only counts as punctuation if the URL doesn't open it
// itself, so that "https://x/Pikachu_(Electric)" keeps its parenthesis.
func trimmedLen(url []byte) int {
	// Never trim into the scheme.
	floor := 1
	if i := bytes.Index(url, []byte("://")); i >= 0 {
		floor = i + len("://") + 1
	}

	end := len(url)
	for end > floor {
		c := url[end-1]
		switch {
		case bytes.IndexByte([]byte(".,:;!?"), c) >= 0:
		case c == '\'' || c == '"':
			if bytes.Count(url[:end], []byte{c})%2 == 0 {
				return end
			}
		default:
			opener, ok := bracketOpener(c)
			if !ok || bytes.Count(url[:end], []byte{c}) <= bytes.Count(url[:end], []byte{opener}) {
				return end
			}
		}
		end--
	}
	return end
}

func bracketOpener(closer byte) (byte, bool) {
	switch closer {
	case ')':
		return '(', true
	case ']':
		return '[', true
	case '}':
		return '{', true
	}
	return 0, false
}
