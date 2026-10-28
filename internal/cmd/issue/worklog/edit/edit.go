package edit

import "github.com/rethab/jira-cli/pkg/jira"

type editParams struct {
	issueKey    string
	worklogID   string
	timeSpent   string
	started     string
	comment     *string
	newEstimate string
	noInput     bool
}

func (p *editParams) nothingToUpdate() bool {
	return false
}

func (p *editParams) merge(_ *jira.Worklog) jira.WorklogUpdate {
	return jira.WorklogUpdate{}
}
