package cmdcommon

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/rethab/jira-cli/internal/mention"
	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/md"
)

const (
	replyQuoteLines = 3
	replyQuoteRunes = 300
)

// wikiMention matches the Server/Data Center and Cloud wiki markup of a mention.
var wikiMention = regexp.MustCompile(`\[~(?:accountid:)?([^\[\]\n]+)\]`)

// wikiMacroLine matches a line that only opens or closes a wiki macro, like the
// {panel:bgColor=...} the editor shows for a Cloud panel.
var wikiMacroLine = regexp.MustCompile(`^\{[a-zA-Z]+(:[^}]*)?\}$`)

// EditorBody translates a description or comment body for the editor: Cloud
// serves ADF and Server/Data Center wiki markup, and the v2 wiki rendering of
// Cloud's ADF has escapes and smart-link forms FromJiraMD doesn't undo. A smart
// link comes out as its bare URL, which the submit turns back into one. A
// literal @[...] in the stored text is escaped, because the submit would
// otherwise resolve it as a mention and ping someone.
func EditorBody(body any) string {
	if doc, ok := body.(*adf.ADF); ok {
		return mention.Escape(adf.NewTranslator(doc, adf.NewJiraMarkdownTranslator()).Translate())
	}
	s, _ := body.(string)
	return mention.Escape(md.FromJiraMD(s))
}

// ReplyPrefix builds the start of a comment that answers another one: a mention
// of its author and a short quote of what they wrote. Jira's REST API can't
// create a threaded reply, so this is what makes the new comment recognizable
// as one. The prefix ends with a blank line, ready for the reply to follow.
func ReplyPrefix(original *jira.Comment, local bool) string {
	who := original.Author.DisplayName
	if a := &original.Author; (local && a.Name != "") || (!local && a.AccountID != "") {
		who = mention.Markup(a, local)
	}

	quote := replyQuote(quoteSource(original.Body))
	if quote == "" {
		return who + "\n\n"
	}
	return who + " wrote:\n\n" + quote + "\n\n"
}

// quoteSource is EditorBody with the mentions turned into plain names: the
// reply pings its addressee, not everyone the original comment mentioned.
func quoteSource(body any) string {
	if doc, ok := body.(*adf.ADF); ok {
		return mention.Escape(adf.NewTranslator(doc, adf.NewJiraMarkdownTranslator(adf.WithPlainMentions())).Translate())
	}
	s, _ := body.(string)
	return mention.Escape(wikiMention.ReplaceAllString(md.FromJiraMD(s), "@$1"))
}

// replyQuote turns the start of a Markdown body into a block quote.
func replyQuote(markdown string) string {
	var lines []string
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		// A fence or macro would swallow the reply that follows if the quote
		// ends before the code or panel does.
		if line == "" || strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") || wikiMacroLine.MatchString(line) {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}

	cut := len(lines) > replyQuoteLines
	if cut {
		lines = lines[:replyQuoteLines]
	}

	budget := replyQuoteRunes
	var out []string
	for i, line := range lines {
		n := utf8.RuneCountInString(line)
		if n > budget {
			out = append(out, quoteLine(beforeOpenLink(string([]rune(line)[:budget]))+"…"))
			break
		}
		budget -= n
		last := i == len(lines)-1
		if !last && budget == 0 {
			// The next line would hold nothing but the ellipsis.
			out = append(out, quoteLine(line+"…"))
			break
		}
		if cut && last {
			line += "…"
		}
		out = append(out, quoteLine(line))
	}

	return strings.Join(out, "\n")
}

// beforeOpenLink drops a mention or link the truncation cut in half, which
// would otherwise be submitted as escaped brackets instead of pinging or
// linking anything.
func beforeOpenLink(s string) string {
	i := strings.LastIndex(s, "[")
	if i < 0 {
		return s
	}
	rest := s[i:]
	if closing := strings.Index(rest, "]"); closing >= 0 {
		target := rest[closing+1:]
		if !strings.HasPrefix(target, "(") || strings.Contains(target, ")") {
			return s
		}
	}
	return strings.TrimRight(s[:i], " ")
}

// quoteLine keeps a literal @[...] in the quoted text from being resolved as a
// mention of someone. Dropping fences can expose one that was in code.
func quoteLine(line string) string {
	return "> " + mention.Escape(line)
}
