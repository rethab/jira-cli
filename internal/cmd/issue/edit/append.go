package edit

import (
	"errors"
	"regexp"
	"strings"

	"github.com/rethab/jira-cli/pkg/jira"
)

var (
	wikiBulletLine   = regexp.MustCompile(`^\s*[*-]+\s`)
	wikiNumberedLine = regexp.MustCompile(`^\s*#+\s`)
)

func checkDescriptionAdditions(params *editParams) error {
	if err := checkAddition("--append-body", params.appendBody, params.appendBodySet, params); err != nil {
		return err
	}
	return checkAddition("--prepend-body", params.prependBody, params.prependBodySet, params)
}

func checkAddition(flag, value string, set bool, params *editParams) error {
	if !set {
		return nil
	}
	if params.body != "" || params.template != "" {
		return errors.New(flag + " can't be combined with --body or --template")
	}
	if strings.TrimSpace(value) == "" {
		return errors.New(flag + " is empty")
	}
	return nil
}

// Adding to the description must never fall back to stdin: a non-TTY stdin is
// common in scripts and agents, and its content would replace the description
// instead.
func shouldReadBodyFromStdin(params *editParams, stdinHasData bool) bool {
	return params.body == "" && params.template == "" &&
		params.appendBody == "" && params.prependBody == "" && stdinHasData
}

// The description write is a request of its own, so an edit request with nothing
// else in it would be a second, empty update that Jira can still announce to
// watchers. The assignee is set by a separate request and doesn't count.
func onlyAddingToDescription(params *editParams) bool {
	return (params.appendBody != "" || params.prependBody != "") &&
		params.summary == "" && params.body == "" && params.issueType == "" &&
		params.parentIssueKey == "" && params.priority == "" && params.originalEstimate == "" &&
		len(params.labels) == 0 && len(params.components) == 0 &&
		len(params.fixVersions) == 0 && len(params.affectsVersions) == 0 &&
		len(params.customFields) == 0
}

// Cloud keeps the description as ADF and Server as wiki markup, so each is
// extended in its own format; converting the existing content would change it.
// Both ends are applied to a single read so that they land in one update.
func extendDescription(client *jira.Client, key, prepend, appendMd string, local, skipNotify bool) error {
	if local {
		wiki, err := client.GetIssueDescriptionV2(key)
		if err != nil {
			return err
		}
		if prepend != "" {
			wiki = prependWiki(wiki, client.ToJiraMD(prepend))
		}
		if appendMd != "" {
			wiki = appendWiki(wiki, client.ToJiraMD(appendMd))
		}
		return client.SetIssueDescriptionV2(key, wiki, skipNotify)
	}

	doc, err := client.GetIssueDescription(key)
	if err != nil {
		return err
	}
	if prepend != "" {
		doc = doc.Prepend(client.ToADF(prepend))
	}
	if appendMd != "" {
		doc = doc.Append(client.ToADF(appendMd))
	}
	return client.SetIssueDescription(key, doc, skipNotify)
}

type wikiLineKind int

const (
	wikiOther wikiLineKind = iota
	wikiBullet
	wikiNumbered
)

func lineKind(line string) wikiLineKind {
	switch {
	case wikiBulletLine.MatchString(line):
		return wikiBullet
	case wikiNumberedLine.MatchString(line):
		return wikiNumbered
	}
	return wikiOther
}

// Wiki markup ends a list at a blank line, so list items that should extend a
// trailing list must follow it on the very next line.
func appendWiki(existing, added string) string {
	existing = strings.TrimRight(existing, "\r\n")
	if strings.TrimSpace(existing) == "" {
		return added
	}
	if added == "" {
		return existing
	}

	lastLine := existing[strings.LastIndex(existing, "\n")+1:]
	firstLine, _, _ := strings.Cut(added, "\n")

	if kind := lineKind(lastLine); kind != wikiOther && kind == lineKind(firstLine) {
		return existing + "\n" + added
	}
	return existing + "\n\n" + added
}

// The mirror of appendWiki: items that should extend a leading list must sit on
// the line right above it.
func prependWiki(existing, added string) string {
	existing = strings.TrimLeft(existing, "\r\n")
	if strings.TrimSpace(existing) == "" {
		return strings.TrimRight(added, "\r\n")
	}
	return appendWiki(added, existing)
}
