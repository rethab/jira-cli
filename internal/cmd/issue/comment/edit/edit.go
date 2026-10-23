package edit

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdcommon"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/md"
	"github.com/rethab/jira-cli/pkg/surveyext"
)

const (
	helpText = `Edit replaces the body of a comment.

The body supports the same Markdown as 'jira issue comment add'. Whether the
comment is internal and who can see it is left as it is.

Use 'jira issue comment list' to find the id of a comment.`
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
$ jira issue comment edit ISSUE-1 10042 "comment from arg" --template /path/to/template.tmpl`
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

	noInput, err := cmd.Flags().GetBool("no-input")
	cmdutil.ExitIfError(err)

	if cmdutil.StdinHasData() || template == "-" {
		noInput = true
	}

	client := api.DefaultClient(viper.GetBool("debug"))

	if body == "" {
		body = flagBody
	}

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

		return client.EditIssueComment(issueKey, commentID, body)
	}()
	cmdutil.ExitIfError(err)

	cmdutil.Success("Comment %q of issue %q updated", commentID, issueKey)
	fmt.Println(cmdutil.GenerateServerBrowseURL(viper.GetString("server"), issueKey))
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
				Default:       editorBody(existing.Body),
				HideDefault:   true,
				AppendDefault: true,
			},
			BlankAllowed: false,
		},
	}}
	cmdutil.ExitIfError(survey.Ask(qs, &ans))

	return ans.Body
}

// editorBody translates the body like 'jira issue edit' does for a description:
// Cloud serves ADF and Server/Data Center wiki markup, and the v2 wiki rendering
// of Cloud's ADF has escapes and smart-link forms FromJiraMD doesn't undo.
// Unlike there, the markup is kept to ASCII, because whatever decoration the
// translator adds, such as the pin before a smart link, would be saved back
// into the comment.
func editorBody(body any) string {
	if doc, ok := body.(*adf.ADF); ok {
		return adf.NewTranslator(doc, adf.NewJiraMarkdownTranslator(adf.WithMarkdownASCII())).Translate()
	}
	s, _ := body.(string)
	return md.FromJiraMD(s)
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
