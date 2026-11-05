package remove

import (
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
)

const (
	helpText = `Remove issues from sprint.

Issues are moved to the backlog, i.e. removed from all active and future sprints.`
	examples = `$ jira sprint remove ISSUE-1 ISSUE-2`
)

// NewCmdRemove is a remove command.
func NewCmdRemove() *cobra.Command {
	return &cobra.Command{
		Use:     "remove ISSUE-1 [...ISSUE-N]",
		Short:   "Remove issues from sprint",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"rm", "unassign"},
		Annotations: map[string]string{
			"help:args": "ISSUE-1 [...ISSUE-N]\tKey of the issues to move to the backlog, removing them from all active and future sprints (max 50 issues at once)",
		},
		Run: remove,
	}
}

func remove(_ *cobra.Command, args []string) {
	server := viper.GetString("server")
	project := viper.GetString("project.key")
	params := parseFlags(args, project)
	client := api.DefaultClient(params.debug)

	qs := getQuestions(params)
	if len(qs) > 0 {
		ans := struct {
			Issues string
		}{}
		err := survey.Ask(qs, &ans)
		cmdutil.ExitIfError(err)

		parts := strings.Split(ans.Issues, ",")
		issues := make([]string, 0, len(parts))
		for _, iss := range parts {
			// A trailing comma or blank entry would send an empty key, and Jira rejects the whole batch.
			if iss = strings.TrimSpace(iss); iss != "" {
				issues = append(issues, cmdutil.GetJiraIssueKey(project, iss))
			}
		}
		if len(issues) == 0 {
			cmdutil.Failed("No issue keys given")
		}
		params.issues = issues
	}

	err := func() error {
		s := cmdutil.Info("Removing issues from sprint...")
		defer s.Stop()

		return client.SprintIssuesRemove(params.issues...)
	}()
	cmdutil.ExitIfError(err)

	cmdutil.Success("Issues moved to the backlog\n%s", cmdutil.GenerateServerBrowseURL(server, project))
}

func parseFlags(args []string, project string) *removeParams {
	issues := make([]string, 0, len(args))
	for _, iss := range args {
		issues = append(issues, cmdutil.GetJiraIssueKey(project, iss))
	}

	return &removeParams{
		issues: issues,
		debug:  viper.GetBool("debug"),
	}
}

func getQuestions(params *removeParams) []*survey.Question {
	var qs []*survey.Question

	if len(params.issues) == 0 {
		qs = append(qs, &survey.Question{
			Name: "issues",
			Prompt: &survey.Input{
				Message: "Issues",
				Help:    "Comma separated list of issues key to remove. eg: ISSUE-1, ISSUE-2",
			},
			Validate: survey.Required,
		})
	}

	return qs
}

type removeParams struct {
	issues []string
	debug  bool
}
