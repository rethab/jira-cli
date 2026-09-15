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
	helpText = `List lists worklogs of an issue.

Unlike the worklog field embedded in an issue, this command paginates properly,
so it also works for issues with more than 20 worklogs. Entries are shown
newest first by default.`
	examples = `$ jira issue worklog list ISSUE-1

# Show at most 10 entries
$ jira issue worklog list ISSUE-1 --limit 10

# Page through the oldest entries instead, 20 at a time starting from the 40th
$ jira issue worklog list ISSUE-1 --oldest-first --start 40 --limit 20`

	defaultLimit = 50
)

// NewCmdWorklogList is a worklog list command.
func NewCmdWorklogList() *cobra.Command {
	cmd := cobra.Command{
		Use:     "list ISSUE-KEY",
		Short:   "List worklogs of an issue",
		Long:    helpText,
		Example: examples,
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"help:args": "ISSUE-KEY\tIssue key of the source issue, eg: ISSUE-1",
		},
		Run: list,
	}

	cmd.Flags().SortFlags = false

	cmd.Flags().Uint("limit", defaultLimit, "Maximum number of worklogs to show")
	cmd.Flags().Bool("oldest-first", false, "Show worklogs in the order the API returns them (oldest first) instead of newest first")
	cmd.Flags().Uint("start", 0, "Index to start from; only used together with --oldest-first")
	cmd.Flags().Bool("plain", false, "Display output in plain mode")
	cmd.Flags().Bool("csv", false, "Print output in CSV format")

	return &cmd
}

func list(cmd *cobra.Command, args []string) {
	issueKey := cmdutil.GetJiraIssueKey(viper.GetString("project.key"), args[0])

	limit, err := cmd.Flags().GetUint("limit")
	cmdutil.ExitIfError(err)

	oldestFirst, err := cmd.Flags().GetBool("oldest-first")
	cmdutil.ExitIfError(err)

	start, err := cmd.Flags().GetUint("start")
	cmdutil.ExitIfError(err)

	if cmd.Flags().Changed("start") && !oldestFirst {
		cmdutil.Failed("--start requires --oldest-first")
	}

	plain, err := cmd.Flags().GetBool("plain")
	cmdutil.ExitIfError(err)

	csv, err := cmd.Flags().GetBool("csv")
	cmdutil.ExitIfError(err)

	debug := viper.GetBool("debug")
	client := api.DefaultClient(debug)

	worklogs, err := func() ([]jira.Worklog, error) {
		s := cmdutil.Info("Fetching worklogs")
		defer s.Stop()

		if oldestFirst {
			page, err := client.GetIssueWorklogs(issueKey, start, limit)
			if err != nil {
				return nil, err
			}
			return page.Worklogs, nil
		}

		return newestWorklogs(client, issueKey, limit)
	}()
	cmdutil.ExitIfError(err)

	if len(worklogs) == 0 {
		cmdutil.Failed("No worklogs found for issue %q", issueKey)
		return
	}

	v := view.NewIssueWorklogs(worklogs, view.WorklogDisplayFormat{Plain: plain, CSV: csv})
	cmdutil.ExitIfError(v.Render())
}

// newestWorklogs returns up to limit worklogs for key, newest first. The
// worklog endpoint returns entries oldest first with no way to reverse the
// order server-side, so this jumps to the end of the result set instead of
// paginating through everything from the start.
func newestWorklogs(client *jira.Client, key string, limit uint) ([]jira.Worklog, error) {
	first, err := client.GetIssueWorklogs(key, 0, 1)
	if err != nil {
		return nil, err
	}
	if first.Total == 0 {
		return nil, nil
	}

	startAt := uint(0)
	if uint(first.Total) > limit {
		startAt = uint(first.Total) - limit
	}

	page, err := client.GetIssueWorklogs(key, startAt, limit)
	if err != nil {
		return nil, err
	}

	worklogs := page.Worklogs
	for i, j := 0, len(worklogs)-1; i < j; i, j = i+1, j-1 {
		worklogs[i], worklogs[j] = worklogs[j], worklogs[i]
	}
	return worklogs, nil
}
