// Package mention turns @[Name] references in a Markdown body into Jira user
// mentions.
package mention

import (
	"fmt"
	"strings"

	"github.com/rethab/jira-cli/pkg/jira"
)

// Lookup returns the users that match query.
type Lookup func(query string) ([]*jira.User, error)

// Pick chooses one of several candidates for a mention. A nil Pick makes an
// ambiguous mention an error, which is what a non-interactive run needs.
type Pick func(query string, candidates []*jira.User) (*jira.User, error)

// span is a @[query] reference found in the body, from start to end (exclusive).
type span struct {
	start, end int
	query      string
}

// Resolve replaces every @[query] in body with the Jira markup for the user it
// names: [~accountid:ID] on Cloud and [~name] on Server/Data Center (local).
// Nothing is replaced unless every reference resolves, because a body posted
// with a half-resolved mention would ping nobody and leave a literal @[...].
func Resolve(body string, local bool, lookup Lookup, pick Pick) (string, error) {
	spans := find(body)
	if len(spans) == 0 {
		return body, nil
	}

	resolved := make(map[string]string)
	for _, sp := range spans {
		key := strings.ToLower(sp.query)
		if _, ok := resolved[key]; ok {
			continue
		}

		user, err := resolve(sp.query, lookup, pick)
		if err != nil {
			return "", err
		}
		resolved[key] = Markup(user, local)
	}

	var out strings.Builder
	last := 0
	for _, sp := range spans {
		out.WriteString(body[last:sp.start])
		out.WriteString(resolved[strings.ToLower(sp.query)])
		last = sp.end
	}
	out.WriteString(body[last:])

	return out.String(), nil
}

// Escape keeps the references in body from being resolved. It is for text
// that was stored as it is, where a literal @[...] was never meant as a mention,
// and would otherwise ping someone the next time the text is submitted.
func Escape(body string) string {
	spans := find(body)
	if len(spans) == 0 {
		return body
	}

	var out strings.Builder
	last := 0
	for _, sp := range spans {
		out.WriteString(body[last : sp.start+1])
		out.WriteString(`\`)
		last = sp.start + 1
	}
	out.WriteString(body[last:])

	return out.String()
}

// Markup is the Jira markup that mentions u: [~accountid:ID] on Cloud and
// [~name] on Server/Data Center (local).
func Markup(u *jira.User, local bool) string {
	if local {
		return "[~" + u.Name + "]"
	}
	return "[~accountid:" + u.AccountID + "]"
}

// Label names a user among the candidates of an ambiguous mention.
func Label(u *jira.User) string {
	if u.Email == "" {
		return u.DisplayName
	}
	return u.DisplayName + " <" + u.Email + ">"
}

// ID tells apart users whose labels are the same.
func ID(u *jira.User) string {
	if u.AccountID != "" {
		return u.AccountID
	}
	return u.Name
}

func resolve(query string, lookup Lookup, pick Pick) (*jira.User, error) {
	found, err := lookup(query)
	if err != nil {
		return nil, fmt.Errorf("looking up mention @[%s]: %w", query, err)
	}

	var active, exact []*jira.User
	for _, u := range found {
		if u == nil || !u.Active {
			continue
		}
		active = append(active, u)
		if strings.EqualFold(u.DisplayName, query) || strings.EqualFold(u.Email, query) || strings.EqualFold(u.Name, query) {
			exact = append(exact, u)
		}
	}

	switch {
	case len(exact) == 1:
		return exact[0], nil
	case len(active) == 0:
		return nil, fmt.Errorf("no user found for mention @[%s]", query)
	case len(active) == 1 && strings.Contains(query, "@"):
		// Cloud hides most email addresses, so a search by email can't be
		// confirmed by an exact match. Jira finding a single user is the
		// best evidence there is. A partial name match is not: @[Jane Doe]
		// mustn't silently ping Jane Doerr.
		return active[0], nil
	}

	// Several users share the exact name, so the ones that merely contain it
	// are noise.
	candidates := active
	if len(exact) > 1 {
		candidates = exact
	}

	if pick == nil {
		return nil, fmt.Errorf("mention @[%s] doesn't match one user exactly, use the email or full name of one of:\n%s", query, describe(candidates))
	}

	u, err := pick(query, candidates)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("no user chosen for mention @[%s]", query)
	}
	return u, nil
}

func describe(users []*jira.User) string {
	lines := make([]string, 0, len(users))
	for _, u := range users {
		lines = append(lines, fmt.Sprintf("  %s (%s)", Label(u), ID(u)))
	}
	return strings.Join(lines, "\n")
}

// find returns the references outside of code, in order. It follows the parts
// of CommonMark that matter for that: fenced and indented code blocks and
// code spans.
func find(body string) []span {
	var (
		spans  []span
		offset int
		fence  string
		// An indented line continues an open paragraph or list item, and only
		// starts a code block otherwise.
		paragraph bool
		inList    bool
	)

	for _, line := range strings.SplitAfter(body, "\n") {
		start := offset
		offset += len(line)

		trimmed := strings.TrimRight(line, "\r\n")

		if fence != "" {
			if closesFence(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if f := opensFence(trimmed); f != "" {
			fence = f
			paragraph = false
			continue
		}

		if strings.TrimSpace(trimmed) == "" {
			paragraph = false
			continue
		}
		indented := strings.HasPrefix(trimmed, "    ") || strings.HasPrefix(trimmed, "\t")
		if indented && !paragraph && !inList {
			continue
		}
		if !indented {
			inList = isListItem(trimmed)
		}
		paragraph = true

		spans = append(spans, scanLine(trimmed, start)...)
	}

	return spans
}

// maxListNumberDigits is the longest number CommonMark accepts for an ordered
// list item.
const maxListNumberDigits = 9

// isListItem reports whether line starts a bullet or ordered list item.
func isListItem(line string) bool {
	s := strings.TrimLeft(line, " ")
	if len(line)-len(s) > 3 || s == "" {
		return false
	}

	n := 0
	switch {
	case strings.ContainsRune("-*+", rune(s[0])):
		n = 1
	default:
		for n < len(s) && n < maxListNumberDigits && s[n] >= '0' && s[n] <= '9' {
			n++
		}
		if n == 0 || n == len(s) || (s[n] != '.' && s[n] != ')') {
			return false
		}
		n++
	}
	return n == len(s) || s[n] == ' ' || s[n] == '\t'
}

func opensFence(line string) string {
	s := strings.TrimLeft(line, " ")
	if len(line)-len(s) > 3 {
		return ""
	}
	for _, c := range []string{"`", "~"} {
		n := len(s) - len(strings.TrimLeft(s, c))
		if n < 3 {
			continue
		}
		// A backtick fence's info string can't hold a backtick, otherwise it
		// is an inline code span that happens to start the line.
		if c == "`" && strings.Contains(s[n:], "`") {
			return ""
		}
		return strings.Repeat(c, n)
	}
	return ""
}

func closesFence(line, fence string) bool {
	s := strings.TrimSpace(line)
	return len(s) >= len(fence) && strings.Trim(s, fence[:1]) == ""
}

func scanLine(line string, base int) []span {
	var spans []span

	for i := 0; i < len(line); {
		switch line[i] {
		case '\\':
			i += 2
		case '`':
			i = skipCodeSpan(line, i)
		case '@':
			sp, ok := parseReference(line, i)
			if !ok {
				i++
				continue
			}
			sp.start += base
			sp.end += base
			spans = append(spans, sp)
			i = sp.end - base
		default:
			i++
		}
	}

	return spans
}

// skipCodeSpan returns the index after the code span that opens at i. An
// opening run of backticks without a closing run of the same length is plain
// text.
func skipCodeSpan(line string, i int) int {
	n := runLength(line, i)
	for j := i + n; j < len(line); {
		if line[j] != '`' {
			j++
			continue
		}
		m := runLength(line, j)
		if m == n {
			return j + m
		}
		j += m
	}
	return i + n
}

func runLength(line string, i int) int {
	n := 0
	for i+n < len(line) && line[i+n] == '`' {
		n++
	}
	return n
}

// parseReference parses @[query] at i. A reference followed by "(" or "[" is
// the text of a Markdown inline or reference link that happens to follow an
// @, and is left alone.
func parseReference(line string, i int) (span, bool) {
	if !strings.HasPrefix(line[i:], "@[") {
		return span{}, false
	}
	closing := strings.IndexAny(line[i+2:], "[]")
	if closing < 0 || line[i+2+closing] != ']' {
		return span{}, false
	}
	end := i + 2 + closing + 1

	query := strings.TrimSpace(line[i+2 : end-1])
	if query == "" || strings.HasPrefix(line[end:], "(") || strings.HasPrefix(line[end:], "[") {
		return span{}, false
	}
	return span{start: i, end: end, query: query}, true
}
