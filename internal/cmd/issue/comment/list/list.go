package list

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/internal/view"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `List lists the comments of an issue, oldest first, along with their ids.

Use the id to edit or delete a comment.`
	examples = `$ jira issue comment list ISSUE-1

# Print in plain mode
$ jira issue comment list ISSUE-1 --plain

# Plain mode without headers, eg: for scripts
$ jira issue comment list ISSUE-1 --plain --no-headers

# Print in CSV format
$ jira issue comment list ISSUE-1 --csv

# Print the raw API response
$ jira issue comment list ISSUE-1 --raw`
)

// NewCmdCommentList is a comment list command.
func NewCmdCommentList() *cobra.Command {
	cmd := cobra.Command{
		Use:     "list ISSUE-KEY",
		Short:   "List comments of an issue",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"help:args": "ISSUE-KEY\tIssue key of the source issue, eg: ISSUE-1",
		},
		Run: list,
	}

	cmd.Flags().SortFlags = false

	cmd.Flags().Bool("plain", false, "Display output in plain mode")
	cmd.Flags().Bool("csv", false, "Print output in CSV format")
	cmd.Flags().Bool("no-headers", false, "Don't display table headers in plain mode. Works only with --plain")
	cmd.Flags().Bool("raw", false, "Print the comments as raw JSON, as returned by the Jira API")

	return &cmd
}

func list(cmd *cobra.Command, args []string) {
	issueKey := cmdutil.GetJiraIssueKey(viper.GetString("project.key"), args[0])

	plain, err := cmd.Flags().GetBool("plain")
	cmdutil.ExitIfError(err)

	csv, err := cmd.Flags().GetBool("csv")
	cmdutil.ExitIfError(err)

	raw, err := cmd.Flags().GetBool("raw")
	cmdutil.ExitIfError(err)

	noHeaders, err := cmd.Flags().GetBool("no-headers")
	cmdutil.ExitIfError(err)

	client := api.DefaultClient(viper.GetBool("debug"))

	comments, err := func() ([]jira.Comment, error) {
		s := cmdutil.Info("Fetching comments")
		defer s.Stop()

		return client.GetIssueComments(issueKey)
	}()
	cmdutil.ExitIfError(err)

	if raw {
		cmdutil.OutputRawJSON(comments)
		return
	}

	if len(comments) == 0 {
		cmdutil.Failed("No comments found for issue %q", issueKey)
		return
	}

	v := view.NewIssueComments(comments, view.ListDisplayFormat{Plain: plain, CSV: csv, NoHeaders: noHeaders})
	cmdutil.ExitIfError(v.Render())
}
