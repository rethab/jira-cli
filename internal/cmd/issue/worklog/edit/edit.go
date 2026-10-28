package edit

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdcommon"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/md"
	"github.com/rethab/jira-cli/pkg/surveyext"
)

const (
	helpText = `Edit changes the time spent, start date or comment of a worklog.

Whatever you don't change, including who can see the worklog, is left as it is.
The comment supports the same Markdown as 'jira issue worklog add'.

Use 'jira issue worklog list' to find the id of a worklog.`
	examples = `# Prompt for the time spent and edit the comment in your editor
$ jira issue worklog edit ISSUE-1 10042

# Pass the new time spent to skip its prompt
$ jira issue worklog edit ISSUE-1 10042 "2d 1h 30m"

# Change only the comment, without any prompt for scripts
$ jira issue worklog edit ISSUE-1 10042 --comment "Fixed comment" --no-input

# 'update' is an alias of 'edit'
$ jira issue worklog update ISSUE-1 10042 3h --no-input

# Change the start date (defaults to UTC timezone)
$ jira issue worklog edit ISSUE-1 10042 --started "2022-01-01 09:30:00" --no-input

# You can specify timezone to use along with the start date in IANA timezone format
$ jira issue worklog edit ISSUE-1 10042 --started "2022-01-01 09:30:00" --timezone "Europe/Berlin" --no-input

# Set the remaining estimate of the issue instead of letting Jira adjust it
$ jira issue worklog edit ISSUE-1 10042 3h --new-estimate "2d 4h" --no-input`
)

// NewCmdWorklogEdit is a worklog edit command.
func NewCmdWorklogEdit() *cobra.Command {
	cmd := cobra.Command{
		Use:     "edit ISSUE-KEY WORKLOG_ID [TIME_SPENT]",
		Short:   "Edit a worklog of an issue",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"update"},
		Args:    cobra.RangeArgs(2, 3),
		Annotations: map[string]string{
			"help:args": "ISSUE-KEY\tIssue key of the source issue, eg: ISSUE-1\n" +
				"WORKLOG_ID\tId of the worklog, see 'jira issue worklog list'\n" +
				"TIME_SPENT\tNew time spent as days (d), hours (h), or minutes (m), separated by space eg: 2d 1h 30m",
		},
		Run: edit,
	}

	cmd.Flags().SortFlags = false

	cmd.Flags().String("started", "", "The new datetime on which the worklog effort was started, eg: 2022-01-01 09:30:00")
	cmd.Flags().String("timezone", "UTC", "The timezone to use for the started date in IANA timezone format, eg: Europe/Berlin")
	cmd.Flags().String("comment", "", "New comment about the worklog")
	cmd.Flags().String("new-estimate", "", "The new remaining estimate of the issue, eg: 2d 4h")
	cmd.Flags().Bool("no-input", false, "Disable prompt for non-required fields")

	return &cmd
}

func edit(cmd *cobra.Command, args []string) {
	params := parseArgsAndFlags(args, cmd.Flags())

	if params.noInput && params.nothingToUpdate() {
		cmdutil.Failed("Nothing to update: pass a time spent, --started, --comment or --new-estimate")
	}

	client := api.DefaultClient(viper.GetBool("debug"))

	existing, err := func() (*jira.Worklog, error) {
		s := cmdutil.Info("Fetching worklog")
		defer s.Stop()

		return client.GetIssueWorklog(params.issueKey, params.worklogID)
	}()
	cmdutil.ExitIfError(err)

	if !params.noInput {
		cmdutil.ExitIfError(params.ask(existing))

		answer := struct{ Action string }{}
		cmdutil.ExitIfError(survey.Ask([]*survey.Question{getNextAction()}, &answer))

		if answer.Action == cmdcommon.ActionCancel {
			cmdutil.Failed("Action aborted")
		}
	}

	_, err = func() (*jira.Worklog, error) {
		s := cmdutil.Info("Updating worklog")
		defer s.Stop()

		return client.UpdateIssueWorklog(params.issueKey, params.worklogID, params.merge(existing), params.newEstimate)
	}()
	cmdutil.ExitIfError(err)

	cmdutil.Success("Worklog %q of issue %q updated", params.worklogID, params.issueKey)
	fmt.Println(cmdutil.GenerateServerBrowseURL(viper.GetString("server"), params.issueKey))
}

type editParams struct {
	issueKey    string
	worklogID   string
	timeSpent   string
	started     string
	comment     *string // nil leaves the comment as it is, unlike an empty one
	newEstimate string
	noInput     bool
}

func parseArgsAndFlags(args []string, flags *pflag.FlagSet) *editParams {
	params := editParams{
		issueKey:  cmdutil.GetJiraIssueKey(viper.GetString("project.key"), args[0]),
		worklogID: args[1],
	}
	if len(args) > 2 {
		params.timeSpent = args[2]
	}

	started, err := flags.GetString("started")
	cmdutil.ExitIfError(err)

	timezone, err := flags.GetString("timezone")
	cmdutil.ExitIfError(err)

	params.started, err = cmdutil.DateStringToJiraFormatInLocation(started, timezone)
	cmdutil.ExitIfError(err)

	comment, err := flags.GetString("comment")
	cmdutil.ExitIfError(err)

	if flags.Changed("comment") {
		params.comment = &comment
	}

	params.newEstimate, err = flags.GetString("new-estimate")
	cmdutil.ExitIfError(err)

	params.noInput, err = flags.GetBool("no-input")
	cmdutil.ExitIfError(err)

	return &params
}

func (p *editParams) nothingToUpdate() bool {
	return p.timeSpent == "" && p.started == "" && p.comment == nil && p.newEstimate == ""
}

// merge applies the changes on top of the existing worklog. The update
// overwrites the worklog with what is sent, so everything that didn't change
// has to be sent back as it is.
func (p *editParams) merge(existing *jira.Worklog) jira.WorklogUpdate {
	update := jira.WorklogUpdate{
		Started:    existing.Started,
		TimeSpent:  existing.TimeSpent,
		Comment:    existing.Comment,
		Visibility: existing.Visibility,
	}

	if p.timeSpent != "" {
		update.TimeSpent = p.timeSpent
	}
	if p.started != "" {
		update.Started = p.started
	}
	if p.comment != nil {
		update.Comment = md.ToJiraMD(*p.comment)
	}

	return update
}

// ask prompts for what wasn't passed, starting from the current values. The
// comment is only taken over when it differs from what the editor started
// with, so skipping the prompt doesn't round-trip it through Markdown.
func (p *editParams) ask(existing *jira.Worklog) error {
	var qs []*survey.Question

	if p.timeSpent == "" {
		qs = append(qs, &survey.Question{
			Name: "timeSpent",
			Prompt: &survey.Input{
				Message: "Time spent",
				Default: existing.TimeSpent,
				Help:    "Time to log as days (d), hours (h), or minutes (m), separated by space eg: 2d 1h 30m",
			},
			Validate: survey.Required,
		})
	}

	current := md.FromJiraMD(existing.Comment)
	if p.comment == nil {
		qs = append(qs, &survey.Question{
			Name: "comment",
			Prompt: &surveyext.JiraEditor{
				Editor: &survey.Editor{
					Message:       "Comment body",
					Default:       current,
					HideDefault:   true,
					AppendDefault: true,
				},
				BlankAllowed: true,
			},
		})
	}

	if len(qs) == 0 {
		return nil
	}

	ans := struct{ TimeSpent, Comment string }{}
	if err := survey.Ask(qs, &ans); err != nil {
		return err
	}

	if ans.TimeSpent != "" {
		p.timeSpent = ans.TimeSpent
	}
	if p.comment == nil && ans.Comment != current {
		p.comment = &ans.Comment
	}

	return nil
}

func getNextAction() *survey.Question {
	return &survey.Question{
		Name: "action",
		Prompt: &survey.Select{
			Message: "What's next?",
			Options: []string{
				cmdcommon.ActionSubmit,
				cmdcommon.ActionCancel,
			},
		},
		Validate: survey.Required,
	}
}
