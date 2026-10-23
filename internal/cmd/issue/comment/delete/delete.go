package delete

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
)

const (
	helpText = `Delete deletes a comment from an issue.

Deleting a comment can't be undone, so you are asked to confirm first.
Use '--no-input' to skip the confirmation, eg: in scripts.

Use 'jira issue comment list' to find the id of a comment.`
	examples = `$ jira issue comment delete ISSUE-1 10042

# Skip the confirmation prompt
$ jira issue comment delete ISSUE-1 10042 --no-input`
)

// NewCmdCommentDelete is a comment delete command.
func NewCmdCommentDelete() *cobra.Command {
	cmd := cobra.Command{
		Use:     "delete ISSUE-KEY COMMENT_ID",
		Short:   "Delete a comment of an issue",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"remove", "rm", "del"},
		Args:    cobra.ExactArgs(2),
		Annotations: map[string]string{
			"help:args": "ISSUE-KEY\tIssue key of the source issue, eg: ISSUE-1\n" +
				"COMMENT_ID\tId of the comment, see 'jira issue comment list'",
		},
		Run: del,
	}

	cmd.Flags().Bool("no-input", false, "Delete without asking for confirmation")

	return &cmd
}

func del(cmd *cobra.Command, args []string) {
	issueKey := cmdutil.GetJiraIssueKey(viper.GetString("project.key"), args[0])
	commentID := args[1]

	noInput, err := cmd.Flags().GetBool("no-input")
	cmdutil.ExitIfError(err)

	if !noInput {
		var confirmed bool

		prompt := &survey.Confirm{
			Message: "Delete comment " + commentID + " of issue " + issueKey + "? This can't be undone.",
		}
		cmdutil.ExitIfError(survey.AskOne(prompt, &confirmed))

		if !confirmed {
			cmdutil.Failed("Action aborted")
		}
	}

	client := api.DefaultClient(viper.GetBool("debug"))

	err = func() error {
		s := cmdutil.Info("Deleting comment")
		defer s.Stop()

		return client.DeleteIssueComment(issueKey, commentID)
	}()
	cmdutil.ExitIfError(err)

	cmdutil.Success("Comment %q deleted from issue %q", commentID, issueKey)
}
