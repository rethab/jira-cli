package delete

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
)

const (
	helpText = `Delete deletes a worklog from an issue.

Deleting a worklog can't be undone, so you are asked to confirm first.
Use '--no-input' to skip the confirmation, eg: in scripts.

Use 'jira issue worklog list' to find the id of a worklog.`
	examples = `$ jira issue worklog delete ISSUE-1 10042

# Skip the confirmation prompt
$ jira issue worklog delete ISSUE-1 10042 --no-input

# Set the remaining estimate of the issue instead of letting Jira adjust it
$ jira issue worklog delete ISSUE-1 10042 --new-estimate "2d 4h"`
)

// NewCmdWorklogDelete is a worklog delete command.
func NewCmdWorklogDelete() *cobra.Command {
	cmd := cobra.Command{
		Use:     "delete ISSUE-KEY WORKLOG_ID",
		Short:   "Delete a worklog of an issue",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"remove", "rm", "del"},
		Args:    cobra.ExactArgs(2),
		Annotations: map[string]string{
			"help:args": "ISSUE-KEY\tIssue key of the source issue, eg: ISSUE-1\n" +
				"WORKLOG_ID\tId of the worklog, see 'jira issue worklog list'",
		},
		Run: del,
	}

	cmd.Flags().String("new-estimate", "", "The new remaining estimate of the issue, eg: 2d 4h")
	cmd.Flags().Bool("no-input", false, "Delete without asking for confirmation")

	return &cmd
}

func del(cmd *cobra.Command, args []string) {
	issueKey := cmdutil.GetJiraIssueKey(viper.GetString("project.key"), args[0])
	worklogID := args[1]

	newEstimate, err := cmd.Flags().GetString("new-estimate")
	cmdutil.ExitIfError(err)

	noInput, err := cmd.Flags().GetBool("no-input")
	cmdutil.ExitIfError(err)

	if !noInput {
		var confirmed bool

		prompt := &survey.Confirm{
			Message: "Delete worklog " + worklogID + " of issue " + issueKey + "? This can't be undone.",
		}
		cmdutil.ExitIfError(survey.AskOne(prompt, &confirmed))

		if !confirmed {
			cmdutil.Failed("Action aborted")
		}
	}

	client := api.DefaultClient(viper.GetBool("debug"))

	err = func() error {
		s := cmdutil.Info("Deleting worklog")
		defer s.Stop()

		return client.DeleteIssueWorklog(issueKey, worklogID, newEstimate)
	}()
	cmdutil.ExitIfError(err)

	cmdutil.Success("Worklog %q deleted from issue %q", worklogID, issueKey)
}
