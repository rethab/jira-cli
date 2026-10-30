package jirawiki

import (
	"regexp"
	"strings"
	"unicode"
)

// escapable holds the characters that a backslash escapes in Jira wiki
// markup. It covers everything the markdown to wiki renderer escapes, plus
// the remaining characters Jira treats as markup.
const escapable = "*_-+^~{}!()[]|#?"

// markerBase is the start of the Unicode private use area. An escaped
// character is hidden from the parser as a single rune of that area, so that
// neither the tokenizer nor the markdown rewriting ever sees it as markup.
const markerBase = 0xE000

// blockStart matches what may precede a character on a line of markdown output
// for that character to be read as a block marker, such as a list bullet or
// a heading.
var blockStart = regexp.MustCompile(`^[ \t>]*(?:(?:[-+*]|\d+[.)])[ \t]+)*$`)

// protectEscapes replaces every escaped character outside of code with a
// marker. Backslashes are literal in code, so those regions are left alone.
func protectEscapes(lines []string) []string {
	out := make([]string, len(lines))
	inBlock := false

	for i, line := range lines {
		if inBlock {
			out[i] = line
			if closesCodeBlock(line) {
				inBlock = false
			}
			continue
		}
		out[i], inBlock = protectLine(line)
	}

	return out
}

// closesCodeBlock mirrors how handleFencedCodeBlock finds the end of a block.
func closesCodeBlock(line string) bool {
	line = strings.TrimSpace(line)
	return line == TagCodeBlock || line == TagNoFormat || checkForInlineClose(line) > 0
}

// protectLine protects the escapes of a single line and reports whether the
// line opens a code block that continues on the next lines.
func protectLine(line string) (string, bool) {
	var out strings.Builder

	for i := 0; i < len(line); {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '\\':
			// A double backslash is Jira's line break, not an escaped backslash.
			out.WriteString(`\\`)
			i += 2
		case line[i] == '\\' && i+1 < len(line) && strings.IndexByte(escapable, line[i+1]) >= 0:
			out.WriteRune(markerBase + rune(line[i+1]))
			i += 2
		case strings.HasPrefix(line[i:], "{{"):
			end := len(line)
			if n := strings.Index(line[i+2:], "}}"); n >= 0 {
				end = i + 2 + n + 2
			}
			out.WriteString(line[i:end])
			i = end
		case line[i] == '{':
			if tag := codeTagAt(line[i:]); tag != "" {
				closing := strings.Index(line[i+len(tag):], tag)
				if closing < 0 {
					out.WriteString(line[i:])
					return out.String(), true
				}
				end := i + len(tag) + closing + len(tag)
				out.WriteString(line[i:end])
				i = end
				continue
			}
			out.WriteByte(line[i])
			i++
		default:
			out.WriteByte(line[i])
			i++
		}
	}

	return out.String(), false
}

// codeTagAt returns the closing form of the code macro that opens at the start
// of s, e.g. "{code}" for "{code:java}", or "" if s doesn't open one.
func codeTagAt(s string) string {
	for _, tag := range []string{TagCodeBlock, TagNoFormat} {
		name := tag[:len(tag)-1]
		if strings.HasPrefix(s, name) && len(s) > len(name) && (s[len(name)] == '}' || s[len(name)] == ':') && strings.Contains(s, "}") {
			return tag
		}
	}
	return ""
}

// restoreEscapes turns the markers back into the characters they stand for.
// The output is markdown that is both read by humans and rendered, so a
// character is only kept escaped when dropping the backslash would let it take
// on a meaning in markdown that it doesn't have in the wiki text.
func restoreEscapes(s string) string {
	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	lineStart := 0

	for i, r := range runes {
		if r == newLine {
			out = append(out, r)
			lineStart = len(out)
			continue
		}

		c, ok := unmark(r)
		if !ok {
			out = append(out, r)
			continue
		}

		prefix := string(out[lineStart:])
		if needsMarkdownEscape(c, neighbour(runes, i-1), neighbour(runes, i+1), prefix) {
			out = append(out, '\\')
		}
		out = append(out, c)
	}

	return string(out)
}

func unmark(r rune) (rune, bool) {
	c := r - markerBase
	if c < 0 || c >= unicode.MaxASCII || !strings.ContainsRune(escapable, c) {
		return 0, false
	}
	return c, true
}

func hasMarker(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return r >= markerBase && r <= markerBase+unicode.MaxASCII
	})
}

// neighbour returns the character at index i of runes, looking through
// markers. It returns 0 at the ends of the line.
func neighbour(runes []rune, i int) rune {
	if i < 0 || i >= len(runes) || runes[i] == newLine {
		return 0
	}
	if c, ok := unmark(runes[i]); ok {
		return c
	}
	return runes[i]
}

// needsMarkdownEscape reports whether c must stay escaped in markdown, given
// its neighbours (0 at the ends of the line) and the output of its line so far.
func needsMarkdownEscape(c, prev, next rune, prefix string) bool {
	spaced := (prev != 0 && unicode.IsSpace(prev)) && (next == 0 || unicode.IsSpace(next))

	switch c {
	case '*', '~':
		return !spaced
	case '_':
		intraword := isWordChar(prev) && isWordChar(next)
		return !spaced && !intraword
	case ']':
		// '](' makes a link, '][' and ']:' a reference link and its definition.
		return next == '(' || next == '[' || next == ':'
	case '-', '+', '#':
		return blockStart.MatchString(prefix)
	case '|':
		return strings.HasPrefix(prefix, "|")
	default:
		return false
	}
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
