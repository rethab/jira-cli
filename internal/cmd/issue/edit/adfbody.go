package edit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
)

const notAnADFDocument = `--adf-body must be an Atlassian document: a JSON object with "type": "doc", "version": 1 and a "content" array`

func checkADFBodyInstallation(params *editParams, local bool) error {
	if params.adfBody != "" && local {
		return errors.New("--adf-body needs Jira Cloud; Jira Server and Data Center store the description " +
			"as wiki markup, use --body or --template")
	}
	return nil
}

// Like loadTemplate, this runs before the issue is fetched so that a bad file fails
// with nothing written.
func loadADFBody(params *editParams) error {
	if params.adfBody == "" {
		return nil
	}
	b, err := cmdutil.ReadFile(params.adfBody)
	if err != nil {
		return fmt.Errorf("unable to read --adf-body: %w", err)
	}
	params.adfDoc, params.adfParsed, err = parseADFBody(b)
	return err
}

// Only the root is checked; anything deeper is left to Jira, which validates the
// document. The returned bytes are what gets sent: adf.ADF models only part of
// what Jira accepts, so writing it back out would silently drop the rest.
func parseADFBody(b []byte) (json.RawMessage, *adf.ADF, error) {
	// Some Windows editors and PowerShell redirection write a byte order mark, which
	// json.Valid rejects.
	b = bytes.TrimSpace(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")))
	if len(b) == 0 {
		return nil, nil, errors.New("--adf-body is empty")
	}
	if !json.Valid(b) {
		var syntax any
		return nil, nil, fmt.Errorf("--adf-body is not valid JSON: %w", json.Unmarshal(b, &syntax))
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(b, &root); err != nil || root == nil {
		return nil, nil, errors.New(notAnADFDocument)
	}
	if _, ok := root["fields"]; ok {
		return nil, nil, errors.New("--adf-body takes the description itself, not the whole issue; " +
			"pass .fields.description of jira issue view --raw")
	}
	if !bytes.HasPrefix(bytes.TrimSpace(root["content"]), []byte("[")) {
		return nil, nil, errors.New(notAnADFDocument)
	}

	var doc adf.ADF
	if err := json.Unmarshal(b, &doc); err != nil || doc.DocType != "doc" || doc.Version != 1 {
		return nil, nil, errors.New(notAnADFDocument)
	}

	return json.RawMessage(b), &doc, nil
}

// Rebuilding the content array and forgetting a media node is the easy mistake to make,
// and it deletes the attachment behind it.
func checkADFImageLoss(key string, current, next *adf.ADF, allow bool) error {
	if allow {
		return nil
	}
	kept := next.MediaIDs()
	var dropped []string
	for _, id := range current.MediaIDs() {
		if !slices.Contains(kept, id) && !slices.Contains(dropped, id) {
			dropped = append(dropped, id)
		}
	}
	if len(dropped) == 0 {
		return nil
	}
	return fmt.Errorf(
		"the new description of %s drops %d inline image(s) or attachment(s) of the current one; "+
			"keep their media nodes, or pass --allow-image-loss",
		key, len(dropped),
	)
}

func checkADFBodyImageLoss(issue *jira.Issue, params *editParams) error {
	if params.adfDoc == nil {
		return nil
	}
	current, _ := issue.Fields.Description.(*adf.ADF)
	return checkADFImageLoss(params.issueKey, current, params.adfParsed, params.allowImageLoss)
}

func adfDescriptionWrite(client *jira.Client, key string, doc json.RawMessage) func(skipNotify bool) error {
	return func(skipNotify bool) error { return client.SetIssueDescriptionADF(key, doc, skipNotify) }
}
