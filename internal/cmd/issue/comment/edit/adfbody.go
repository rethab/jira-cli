package edit

import (
	"encoding/json"
	"errors"

	"github.com/rethab/jira-cli/internal/cmdcommon"
)

// adfBodyFlags is what the command was given that decides whether --adf-body can be used.
type adfBodyFlags struct {
	path     string
	body     string // the positional argument or --body
	template string
}

// checkADFBody is only called when --adf-body was given.
func checkADFBody(f adfBodyFlags, local bool) error {
	// An unset shell variable would otherwise turn the flag off and the command into the
	// stdin or editor path.
	if f.path == "" {
		return errors.New("--adf-body needs a path, or - for standard input")
	}
	if f.body != "" || f.template != "" {
		return errors.New("--adf-body can't be combined with a body, --body or --template")
	}
	if local {
		return errors.New("--adf-body needs Jira Cloud; Jira Server and Data Center store comments as wiki markup, " +
			"use --body or --template")
	}
	return nil
}

// Run before any request, so that a bad file fails with nothing written.
func loadADFBody(path string) (json.RawMessage, error) {
	doc, _, err := cmdcommon.ReadADFDocument(path, "--adf-body", ".body of one comment of jira issue comment list --raw")
	return doc, err
}
