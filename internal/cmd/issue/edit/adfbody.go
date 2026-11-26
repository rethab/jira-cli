package edit

import (
	"errors"
	"fmt"
	"slices"

	"github.com/rethab/jira-cli/internal/cmdcommon"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
)

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
	params.adfDoc, params.adfParsed, err = cmdcommon.ParseADFDocument(b, "--adf-body", ".fields.description of jira issue view --raw")
	return err
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
