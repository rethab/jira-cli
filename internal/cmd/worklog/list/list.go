package list

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/internal/view"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `List shows worklogs a user logged across issues on a given day.

Jira has no single endpoint for this: it searches issues via JQL
(worklogAuthor + worklogDate), then fetches each matching issue's worklogs
and filters them down to the given user and day.`
	examples = `$ jira worklog list

# A specific day
$ jira worklog list --date 2022-01-02

# A different user's worklogs
$ jira worklog list --user jdoe@example.com`

	searchLimit = 200
)

// NewCmdWorklogList is a worklog list command.
func NewCmdWorklogList() *cobra.Command {
	cmd := cobra.Command{
		Use:     "list",
		Short:   "List worklogs logged across issues on a given day",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"today"},
		Run:     list,
	}

	cmd.Flags().SortFlags = false

	cmd.Flags().String("date", "", "Day to show worklogs for, in YYYY-MM-DD format (defaults to today)")
	cmd.Flags().String("user", "", "Email of the user to show worklogs for (defaults to the configured user)")
	cmd.Flags().Bool("plain", false, "Display output in plain mode")
	cmd.Flags().Bool("csv", false, "Print output in CSV format")

	return &cmd
}

func list(cmd *cobra.Command, _ []string) {
	date, err := cmd.Flags().GetString("date")
	cmdutil.ExitIfError(err)
	if date == "" {
		date = time.Now().Format(cmdutil.DateLayout)
	}
	if _, err := time.Parse(cmdutil.DateLayout, date); err != nil {
		cmdutil.Failed("Invalid --date %q: expected format YYYY-MM-DD", date)
	}

	user, err := cmd.Flags().GetString("user")
	cmdutil.ExitIfError(err)

	plain, err := cmd.Flags().GetBool("plain")
	cmdutil.ExitIfError(err)

	csv, err := cmd.Flags().GetBool("csv")
	cmdutil.ExitIfError(err)

	debug := viper.GetBool("debug")
	client := api.DefaultClient(debug)

	entries, err := func() ([]view.WorklogReportEntry, error) {
		s := cmdutil.Info("Fetching worklogs")
		defer s.Stop()

		if user == "" {
			me, err := client.Me()
			if err != nil {
				return nil, fmt.Errorf("fetching configured user: %w", err)
			}
			user = me.Email
		}

		return worklogEntries(client, user, date)
	}()
	cmdutil.ExitIfError(err)

	if len(entries) == 0 {
		cmdutil.Failed("No worklogs found for %q on %s", user, date)
		return
	}

	v := view.NewWorklogReport(entries, view.WorklogDisplayFormat{Plain: plain, CSV: csv})
	cmdutil.ExitIfError(v.Render())
}

// worklogEntries finds every worklog user logged on date. It first searches
// for candidate issues via JQL, then fetches each one's worklogs directly: a
// per-issue fetch that fails is returned as an error rather than treated as
// "no time logged", since the two are otherwise indistinguishable and would
// silently understate the total.
func worklogEntries(client *jira.Client, user, date string) ([]view.WorklogReportEntry, error) {
	jql := fmt.Sprintf(`worklogAuthor = %s AND worklogDate = %s`, jqlString(user), jqlString(date))

	result, err := client.Search(jql, searchLimit)
	if err != nil {
		return nil, fmt.Errorf("searching issues: %w", err)
	}
	if !result.IsLast {
		return nil, fmt.Errorf("more than %d issues match %s; narrow the search (this command doesn't paginate the issue search)", searchLimit, jql)
	}

	afterMS, beforeMS, err := dayWindow(date)
	if err != nil {
		return nil, err
	}

	var entries []view.WorklogReportEntry
	for _, issue := range result.Issues {
		page, err := client.GetIssueWorklogsInWindow(issue.Key, afterMS, beforeMS)
		if err != nil {
			return nil, fmt.Errorf("fetching worklogs for %s: %w", issue.Key, err)
		}

		for _, wl := range page.Worklogs {
			if !strings.EqualFold(wl.Author.Email, user) {
				continue
			}
			if len(wl.Started) < 10 || wl.Started[:10] != date {
				continue
			}
			entries = append(entries, view.WorklogReportEntry{
				IssueKey:         issue.Key,
				IssueSummary:     issue.Fields.Summary,
				TimeSpent:        wl.TimeSpent,
				TimeSpentSeconds: wl.TimeSpentSeconds,
				Comment:          wl.Comment,
			})
		}
	}

	return entries, nil
}

// dayWindow returns an epoch millisecond window around date, one day wider on
// each side than the day itself. started is returned using the Jira
// instance's UTC offset, which may not match date's own timezone, so a window
// this wide guarantees every entry for the day is included regardless of
// offset; worklogEntries then filters precisely by the started[0:10] string.
func dayWindow(date string) (afterMS, beforeMS int64, err error) {
	const (
		oneDayMS = 24 * 60 * 60 * 1000
		twoDayMS = 2 * oneDayMS
	)

	day, err := time.ParseInLocation(cmdutil.DateLayout, date, time.UTC)
	if err != nil {
		return 0, 0, err
	}

	epochMS := day.UnixMilli()
	return epochMS - oneDayMS, epochMS + twoDayMS, nil
}

func jqlString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
