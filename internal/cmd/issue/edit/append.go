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

func checkAppendBody(params *editParams) error {
	if !params.appendBodySet {
		return nil
	}
	if params.body != "" || params.template != "" {
		return errors.New("--append-body can't be combined with --body or --template")
	}
	if strings.TrimSpace(params.appendBody) == "" {
		return errors.New("--append-body is empty")
	}
	return nil
}

// Appending must never fall back to stdin: a non-TTY stdin is common in scripts
// and agents, and its content would replace the description instead.
func shouldReadBodyFromStdin(params *editParams, stdinHasData bool) bool {
	return params.body == "" && params.template == "" && params.appendBody == "" && stdinHasData
}

// The append is a request of its own, so an edit request with nothing else in it
// would be a second, empty update that Jira can still announce to watchers.
// The assignee is set by a separate request and doesn't count.
func onlyAppending(params *editParams) bool {
	return params.appendBody != "" &&
		params.summary == "" && params.body == "" && params.issueType == "" &&
		params.parentIssueKey == "" && params.priority == "" && params.originalEstimate == "" &&
		len(params.labels) == 0 && len(params.components) == 0 &&
		len(params.fixVersions) == 0 && len(params.affectsVersions) == 0 &&
		len(params.customFields) == 0
}

// Cloud keeps the description as ADF and Server as wiki markup, so each is
// extended in its own format; converting the existing content would change it.
func appendDescription(client *jira.Client, key, markdown string, local, skipNotify bool) error {
	if local {
		existing, err := client.GetIssueDescriptionV2(key)
		if err != nil {
			return err
		}
		return client.SetIssueDescriptionV2(key, appendWiki(existing, client.ToJiraMD(markdown)), skipNotify)
	}

	existing, err := client.GetIssueDescription(key)
	if err != nil {
		return err
	}
	return client.SetIssueDescription(key, existing.Append(client.ToADF(markdown)), skipNotify)
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
