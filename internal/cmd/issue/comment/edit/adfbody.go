package edit

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rethab/jira-cli/internal/cmdcommon"
	"github.com/rethab/jira-cli/internal/cmdutil"
)

// adfBodyFlags is what the command was given that decides whether --adf-body can be used.
type adfBodyFlags struct {
	set      bool
	path     string
	body     string // the positional argument or --body
	template string
}

func checkADFBody(f adfBodyFlags, local bool) error {
	// An unset shell variable would otherwise turn the flag off and the command into the
	// stdin or editor path.
	if f.set && f.path == "" {
		return errors.New("--adf-body needs a path, or - for standard input")
	}
	if f.path == "" {
		return nil
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
	b, err := cmdutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read --adf-body: %w", err)
	}
	doc, _, err := cmdcommon.ParseADFDocument(b, "--adf-body", ".body of one comment of jira issue comment list --raw")
	return doc, err
}
