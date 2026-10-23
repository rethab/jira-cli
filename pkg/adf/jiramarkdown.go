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
}

// NewJiraMarkdownTranslator constructs jira markdown translator.
func NewJiraMarkdownTranslator(opts ...MarkdownTranslatorOption) *JiraMarkdownTranslator {
	openHooks := nodeTypeHook{
		NodePanel:      nodePanelOpenHook,
		InlineNodeCard: inlineCardOpenHook,
	}

	closeHooks := nodeTypeHook{
		NodePanel: nodePanelCloseHook,
	}

	opts = append([]MarkdownTranslatorOption{
		WithMarkdownOpenHooks(openHooks),
		WithMarkdownCloseHooks(closeHooks),
	}, opts...)

	return &JiraMarkdownTranslator{MarkdownTranslator: NewMarkdownTranslator(opts...)}
}

// Open implements TagOpener interface.
func (tr *JiraMarkdownTranslator) Open(n Connector, d int) string {
	return tr.MarkdownTranslator.Open(n, d)
}

// Close implements TagCloser interface.
func (tr *JiraMarkdownTranslator) Close(n Connector) string {
	return tr.MarkdownTranslator.Close(n)
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
