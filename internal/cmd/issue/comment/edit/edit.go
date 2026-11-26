package edit

import (
	"encoding/json"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdcommon"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/surveyext"
)

const (
	helpText = `Edit replaces the body of a comment.

The body supports the same Markdown as 'jira issue comment add'. Whether the
comment is internal and who can see it is left as it is.

Use 'jira issue comment list' to find the id of a comment. With --adf-body the new
body is an Atlassian document instead (Jira Cloud only), which is sent as is so that
inline cards, mentions, panels and the like survive; read the current one with
'jira issue comment list --raw'.`
	examples = `# Edit the comment in your editor, starting from its current body
$ jira issue comment edit ISSUE-1 10042

# Pass the new body to skip the prompt
$ jira issue comment edit ISSUE-1 10042 "Fixed comment"

# Same as above, with a flag, and without any prompt for scripts
$ jira issue comment edit ISSUE-1 10042 --body "Fixed comment" --no-input

# 'update' is an alias of 'edit'
$ jira issue comment update ISSUE-1 10042 "Fixed comment"

# Load the new body from a template file
$ jira issue comment edit ISSUE-1 10042 --template /path/to/template.tmpl

# Get the new body from standard input
$ jira issue comment edit ISSUE-1 10042 --template -

# Or, use pipe to read input directly from standard input
$ echo "Comment from stdin" | jira issue comment edit ISSUE-1 10042

# Positional argument takes precedence over the body and template flags
$ jira issue comment edit ISSUE-1 10042 "comment from arg" --template /path/to/template.tmpl

# Write the body as an Atlassian document, sent as is (Jira Cloud only); read it with comment list --raw
$ jira issue comment list ISSUE-1 --raw | jq '.[] | select(.id == "10042") | .body' | jira issue comment edit ISSUE-1 10042 --adf-body - --no-input`
)

// NewCmdCommentEdit is a comment edit command.
func NewCmdCommentEdit() *cobra.Command {
	cmd := cobra.Command{
		Use:     "edit ISSUE-KEY COMMENT_ID [COMMENT_BODY]",
		Short:   "Edit a comment of an issue",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"update"},
		Args:    cobra.RangeArgs(2, 3),
		Annotations: map[string]string{
			"help:args": "ISSUE-KEY\tIssue key of the source issue, eg: ISSUE-1\n" +
				"COMMENT_ID\tId of the comment, see 'jira issue comment list'\n" +
				"COMMENT_BODY\tNew body of the comment",
		},
		Run: edit,
	}

	cmd.Flags().String("body", "", "New body of the comment; the positional argument takes precedence")
	cmd.Flags().StringP("template", "T", "", "Path to a file to read comment body from")
	cmd.Flags().String("adf-body", "", "Path to a file with the comment body as an Atlassian document (ADF JSON), "+
		"or - for standard input; sent to Jira as is (Jira Cloud only)")
	cmd.Flags().Bool("no-input", false, "Disable prompt for non-required fields")

	return &cmd
}

func edit(cmd *cobra.Command, args []string) {
	issueKey := cmdutil.GetJiraIssueKey(viper.GetString("project.key"), args[0])
	commentID := args[1]

	var body string
	if len(args) > 2 {
		body = args[2]
	}

	flagBody, err := cmd.Flags().GetString("body")
	cmdutil.ExitIfError(err)

	template, err := cmd.Flags().GetString("template")
	cmdutil.ExitIfError(err)

	adfBody, err := cmd.Flags().GetString("adf-body")
	cmdutil.ExitIfError(err)

	noInput, err := cmd.Flags().GetBool("no-input")
	cmdutil.ExitIfError(err)

	if body == "" {
		body = flagBody
	}

	var adfDoc json.RawMessage
	if cmd.Flags().Changed("adf-body") {
		cmdutil.ExitIfError(checkADFBody(adfBodyFlags{
			path:     adfBody,
			body:     body,
			template: template,
		}, viper.GetString("installation") == jira.InstallationTypeLocal))

		adfDoc, err = loadADFBody(adfBody)
		cmdutil.ExitIfError(err)
	}

	if cmdutil.StdinHasData() || template == "-" || adfBody == "-" {
		noInput = true
	}

	client := api.DefaultClient(viper.GetBool("debug"))

	update := func() error { return client.EditIssueCommentADF(issueKey, commentID, adfDoc) }
	if adfDoc == nil {
		body = markdownBody(client, issueKey, commentID, body, template, noInput)
		update = func() error { return client.EditIssueComment(issueKey, commentID, body) }
	}

	if !noInput {
		answer := struct{ Action string }{}
		cmdutil.ExitIfError(survey.Ask([]*survey.Question{nextAction()}, &answer))

		if answer.Action == cmdcommon.ActionCancel {
			cmdutil.Failed("Action aborted")
		}
	}

	err = func() error {
		s := cmdutil.Info("Updating comment")
		defer s.Stop()

		return update()
	}()
	cmdutil.ExitIfError(err)

	cmdutil.Success("Comment %q of issue %q updated", commentID, issueKey)
	fmt.Println(cmdutil.GenerateServerBrowseURL(viper.GetString("server"), issueKey))
}

// markdownBody returns the new body when it is given as Markdown, from the arguments, the
// template, standard input or the editor, with mentions resolved.
func markdownBody(client *jira.Client, issueKey, commentID, body, template string, noInput bool) string {
	// Stdin is only read when no body was passed, so a script that gives one
	// never blocks on a pipe nobody closes.
	if body == "" && (template != "" || cmdutil.StdinHasData()) {
		b, err := cmdutil.ReadFile(template)
		if err != nil {
			cmdutil.Failed("Error: %s", err)
		}
		body = string(b)
	}

	if body == "" && noInput {
		cmdutil.Failed("Comment body is empty")
	}

	if body == "" {
		body = askBody(client, issueKey, commentID)
	}

	return cmdcommon.ResolveMentions(client, issueKey, "", body, noInput)
}

// askBody opens the editor on the comment's current body. The body is turned
// back into Markdown so that what the user edits is what edit will convert.
func askBody(client *jira.Client, issueKey, commentID string) string {
	existing, err := func() (*jira.Comment, error) {
		s := cmdutil.Info("Fetching comment")
		defer s.Stop()

		return api.ProxyGetIssueComment(client, issueKey, commentID)
	}()
	cmdutil.ExitIfError(err)

	ans := struct{ Body string }{}
	qs := []*survey.Question{{
		Name: "body",
		Prompt: &surveyext.JiraEditor{
			Editor: &survey.Editor{
				Message:       "Comment body",
				Default:       cmdcommon.EditorBody(existing.Body),
				HideDefault:   true,
				AppendDefault: true,
			},
			BlankAllowed: false,
		},
	}}
	cmdutil.ExitIfError(survey.Ask(qs, &ans))

	return ans.Body
}

func nextAction() *survey.Question {
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
