package edit

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/rethab/jira-cli/internal/cmdcommon"
	"github.com/rethab/jira-cli/pkg/jira"
)

var (
	wikiBulletLine   = regexp.MustCompile(`^\s*[*-]+\s`)
	wikiNumberedLine = regexp.MustCompile(`^\s*#+\s`)
)

func checkDescriptionAdditions(params *editParams) error {
	if len(params.replacements) > 0 && (params.body != "" || params.template != "") {
		return errors.New("--replace-body can't be combined with --body or --template")
	}
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

// Changing the description in place must never fall back to stdin: a non-TTY
// stdin is common in scripts and agents, and its content would replace the
// description instead.
func shouldReadBodyFromStdin(params *editParams, stdinHasData bool) bool {
	return params.body == "" && params.template == "" &&
		params.appendBody == "" && params.prependBody == "" && len(params.replacements) == 0 && stdinHasData
}

// The description write is a request of its own, so an edit request with nothing
// else in it would be a second, empty update that Jira can still announce to
// watchers. The assignee is set by a separate request and doesn't count.
func onlyChangingDescription(params *editParams) bool {
	return (params.appendBody != "" || params.prependBody != "" || len(params.replacements) > 0) &&
		params.summary == "" && params.body == "" && params.issueType == "" &&
		params.parentIssueKey == "" && params.priority == "" && params.originalEstimate == "" &&
		len(params.labels) == 0 && len(params.components) == 0 &&
		len(params.fixVersions) == 0 && len(params.affectsVersions) == 0 &&
		len(params.customFields) == 0
}

// replacement is one <search>:<replace> pair of --replace-body.
type replacement struct {
	from, to string
}

func parseReplacements(values []string) ([]replacement, error) {
	var out []replacement
	for _, v := range values {
		from, to, err := cmdcommon.ParseReplace(v)
		if err != nil {
			return nil, fmt.Errorf("invalid --replace-body %q: %w", v, err)
		}
		out = append(out, replacement{from, to})
	}
	return out, nil
}

// descriptionChange is what is done to the existing description. Replacements
// are applied before prepend and append, so that text added in the same command
// is never rewritten.
type descriptionChange struct {
	replacements []replacement
	prepend      string
	appendMd     string
}

// Cloud keeps the description as ADF and Server as wiki markup, so each is
// changed in its own format; converting the existing content would change it.
// Everything is applied to a single read so that it lands in one update, and a
// search that matches nothing fails before anything is written.
func extendDescription(client *jira.Client, key string, change descriptionChange, local, skipNotify bool) error {
	if local {
		wiki, err := client.GetIssueDescriptionV2(key)
		if err != nil {
			return err
		}
		for _, r := range change.replacements {
			if !strings.Contains(wiki, r.from) {
				return fmt.Errorf("%q not found in the description of %s", r.from, key)
			}
			wiki = strings.ReplaceAll(wiki, r.from, r.to)
		}
		if change.prepend != "" {
			wiki = prependWiki(wiki, client.ToJiraMD(change.prepend))
		}
		if change.appendMd != "" {
			wiki = appendWiki(wiki, client.ToJiraMD(change.appendMd))
		}
		return client.SetIssueDescriptionV2(key, wiki, skipNotify)
	}

	doc, err := client.GetIssueDescription(key)
	if err != nil {
		return err
	}
	for _, r := range change.replacements {
		if doc.ReplaceText(r.from, r.to) == 0 {
			return fmt.Errorf(
				"%q not found in the description of %s; text that spans formatting, links, mentions or smart links "+
					"is stored in separate pieces, so search for a part without them",
				r.from, key,
			)
		}
	}
	if change.prepend != "" {
		doc = doc.Prepend(client.ToADF(change.prepend))
	}
	if change.appendMd != "" {
		doc = doc.Append(client.ToADF(change.appendMd))
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
