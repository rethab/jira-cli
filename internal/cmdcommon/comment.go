package cmdcommon

import (
	"strings"
	"unicode/utf8"

	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/md"
)

const (
	replyQuoteLines = 3
	replyQuoteRunes = 300
)

// CommentEditorBody translates a comment body like 'jira issue edit' does for a
// description: Cloud serves ADF and Server/Data Center wiki markup, and the v2
// wiki rendering of Cloud's ADF has escapes and smart-link forms FromJiraMD
// doesn't undo. A smart link comes out as its bare URL, which the submit turns
// back into one.
func CommentEditorBody(body any) string {
	if doc, ok := body.(*adf.ADF); ok {
		return adf.NewTranslator(doc, adf.NewJiraMarkdownTranslator()).Translate()
	}
	s, _ := body.(string)
	return md.FromJiraMD(s)
}

// ReplyPrefix builds the start of a comment that answers another one: a mention
// of its author and a short quote of what they wrote. Jira's REST API can't
// create a threaded reply, so this is what makes the new comment recognizable
// as one. The prefix ends with a blank line, ready for the reply to follow.
func ReplyPrefix(original *jira.Comment, local bool) string {
	var who string
	switch {
	case local && original.Author.Name != "":
		who = "[~" + original.Author.Name + "]"
	case !local && original.Author.AccountID != "":
		who = "[~accountid:" + original.Author.AccountID + "]"
	default:
		who = original.Author.DisplayName
	}

	quote := replyQuote(CommentEditorBody(original.Body))
	if quote == "" {
		return who + "\n\n"
	}
	return who + " wrote:\n\n" + quote + "\n\n"
}

// replyQuote turns the start of a Markdown body into a block quote.
func replyQuote(markdown string) string {
	var lines []string
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		// A fence would swallow the reply that follows if the quote ends before
		// the code does.
		if line == "" || strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
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
			line = string([]rune(line)[:budget]) + "…"
			out = append(out, quoteLine(line))
			break
		}
		budget -= n
		if cut && i == len(lines)-1 {
			line += "…"
		}
		out = append(out, quoteLine(line))
	}

	return strings.Join(out, "\n")
}

// quoteLine keeps a literal "@[" in the quoted text from being resolved as a
// mention of someone.
func quoteLine(line string) string {
	return "> " + strings.ReplaceAll(line, "@[", `@\[`)
}
