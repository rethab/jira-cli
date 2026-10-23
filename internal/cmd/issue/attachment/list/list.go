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
	helpText = `List lists the attachments of an issue along with their ids.`
	examples = `$ jira issue attachment list ISSUE-1

# Print in plain mode
$ jira issue attachment list ISSUE-1 --plain

# Print in CSV format
$ jira issue attachment list ISSUE-1 --csv

# Print the raw attachment data from the API
$ jira issue attachment list ISSUE-1 --raw`
)

// NewCmdAttachmentList is an attachment list command.
func NewCmdAttachmentList() *cobra.Command {
	cmd := cobra.Command{
		Use:     "list ISSUE-KEY",
		Short:   "List attachments of an issue",
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
	cmd.Flags().Bool("raw", false, "Print raw Jira API response")

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

	client := api.DefaultClient(viper.GetBool("debug"))

	attachments, err := func() ([]jira.Attachment, error) {
		s := cmdutil.Info("Fetching attachments")
		defer s.Stop()

		return client.GetIssueAttachments(issueKey)
	}()
	cmdutil.ExitIfError(err)

	if raw {
		cmdutil.OutputRawJSON(attachments)
		return
	}

	if len(attachments) == 0 {
		cmdutil.Failed("No attachments found for issue %q", issueKey)
		return
	}

	v := view.NewIssueAttachments(attachments, view.AttachmentDisplayFormat{Plain: plain, CSV: csv})
	cmdutil.ExitIfError(v.Render())
}
