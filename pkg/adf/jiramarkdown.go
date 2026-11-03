package adf

import (
	"fmt"
	"strings"
)

const (
	bgColorInfo    = "#deebff"
	bgColorNote    = "#eae6ff"
	bgColorError   = "#ffebe6"
	bgColorSuccess = "#e3fcef"
	bgColorWarning = "#fffae6"

	panelTypeInfo    = "info"
	panelTypeNote    = "note"
	panelTypeError   = "error"
	panelTypeSuccess = "success"
	panelTypeWarning = "warning"
)

// JiraMarkdownTranslator is a jira markdown translator.
type JiraMarkdownTranslator struct {
	*MarkdownTranslator
	plainMentions bool
}

// JiraMarkdownOption customizes the jira markdown translator.
type JiraMarkdownOption func(*JiraMarkdownTranslator)

// WithPlainMentions renders mentions as the name they display instead of the
// markup that pings the user, for text that is quoted rather than edited: the
// people mentioned in the original shouldn't be notified again.
func WithPlainMentions() JiraMarkdownOption {
	return func(tr *JiraMarkdownTranslator) {
		tr.plainMentions = true
	}
}

// NewJiraMarkdownTranslator constructs jira markdown translator.
func NewJiraMarkdownTranslator(opts ...JiraMarkdownOption) *JiraMarkdownTranslator {
	openHooks := nodeTypeHook{
		NodePanel:      nodePanelOpenHook,
		InlineNodeCard: inlineCardOpenHook,
	}

	closeHooks := nodeTypeHook{
		NodePanel: nodePanelCloseHook,
	}

	tr := &JiraMarkdownTranslator{
		MarkdownTranslator: NewMarkdownTranslator(
			WithMarkdownOpenHooks(openHooks),
			WithMarkdownCloseHooks(closeHooks),
		),
	}
	for _, opt := range opts {
		opt(tr)
	}
	return tr
}

// Open implements TagOpener interface.
func (tr *JiraMarkdownTranslator) Open(n Connector, d int) string {
	if n.GetType() == InlineNodeMention {
		if tr.plainMentions {
			return mentionText(n)
		}
		return mentionMarkup(n)
	}
	return tr.MarkdownTranslator.Open(n, d)
}

// Close implements TagCloser interface.
func (tr *JiraMarkdownTranslator) Close(n Connector) string {
	return tr.MarkdownTranslator.Close(n)
}

// mentionMarkup renders a mention as the wiki markup that pings the user. The
// editor text is submitted back as the body, so printing the display name, as
// the viewer does, would silently turn the mention into plain text on save.
// Without an id there is nothing to ping, and the display name is all that is left.
func mentionMarkup(n Connector) string {
	attrs, _ := n.GetAttributes().(map[string]any)

	if id, _ := attrs["id"].(string); id != "" {
		return " [~accountid:" + id + "]"
	}
	return mentionText(n)
}

func mentionText(n Connector) string {
	attrs, _ := n.GetAttributes().(map[string]any)
	text, _ := attrs["text"].(string)
	return " " + text
}

// inlineCardOpenHook drops the pin the viewer prints before a card. The editor
// text is submitted back as the description, so the pin would end up in it as a
// literal character, and a further one on every edit.
func inlineCardOpenHook(Connector) string {
	return " "
}

func nodePanelOpenHook(n Connector) string {
	attrs := n.GetAttributes()

	var tag strings.Builder

	tag.WriteString("\n{panel")
	if attrs != nil {
		a := attrs.(map[string]any)
		if len(a) > 0 {
			tag.WriteString(":")
		}
		for k, v := range a {
			if k == "panelType" {
				switch v {
				case panelTypeInfo:
					fmt.Fprintf(&tag, "bgColor=%s", bgColorInfo)
				case panelTypeNote:
					fmt.Fprintf(&tag, "bgColor=%s", bgColorNote)
				case panelTypeError:
					fmt.Fprintf(&tag, "bgColor=%s", bgColorError)
				case panelTypeSuccess:
					fmt.Fprintf(&tag, "bgColor=%s", bgColorSuccess)
				case panelTypeWarning:
					fmt.Fprintf(&tag, "bgColor=%s", bgColorWarning)
				}
			} else {
				fmt.Fprintf(&tag, "|%s=%s", k, v)
			}
		}
	}
	tag.WriteString("}\n")

	return tag.String()
}

func nodePanelCloseHook(Connector) string {
	return "{panel}\n"
}
