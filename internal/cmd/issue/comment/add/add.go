package add

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdcommon"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/internal/query"
	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/surveyext"
)

const (
	helpText = `Add adds comment to an issue and prints the id of the new comment,
which 'jira issue comment edit' and 'jira issue comment delete' take.

Write @[Name] to mention a user. The name, email or (Server/Data Center) username
is looked up among the users who can see the issue and replaced with a real
mention. If several users match, you are asked to choose, or the command fails
when it can't ask. Text in code and Markdown links is left alone.

With --reply-to, the new comment mentions the author of another comment and quotes
the start of it. This is a new top-level comment, not a threaded reply, because
Jira's REST API has no parent comment field. The reply keeps the visibility of the
comment it answers, so an internal comment is never answered in public.`
	examples = `$ jira issue comment add

# Pass required parameters to skip prompt 
$ jira issue comment add ISSUE-1 "My comment"

# Multi-line comment
$ jira issue comment add ISSUE-1 $'Supports\n\nNew line'

# Load comment body from a template file
$ jira issue comment add ISSUE-1 --template /path/to/template.tmpl

# Get comment body from standard input
$ jira issue comment add ISSUE-1 --template -

# Or, use pipe to read input directly from standard input
$ echo "Comment from stdin" | jira issue comment add ISSUE-1

# Positional argument takes precedence over the template flag
# The example below will add "comment from arg" as a comment
$ jira issue comment add ISSUE-1 "comment from arg" --template /path/to/template.tmpl

# Mention a user
$ jira issue comment add ISSUE-1 "@[Jane Doe] can you take a look?"

# Mention and quote the comment with the id 10042 (see 'jira issue comment list')
$ jira issue comment add ISSUE-1 "Thanks, fixed" --reply-to 10042

# Keep the id of the new comment to update it later
$ id=$(jira issue comment add ISSUE-1 "Plan v1" --no-input | sed -n 's/.*(ID: \([0-9]*\)).*/\1/p')
$ jira issue comment update ISSUE-1 "$id" "Plan v2" --no-input`
)

// NewCmdCommentAdd is a comment add command.
func NewCmdCommentAdd() *cobra.Command {
	cmd := cobra.Command{
		Use:     "add ISSUE-KEY [COMMENT_BODY]",
		Short:   "Add a comment to an issue",
		Long:    helpText,
		Example: examples,
		Annotations: map[string]string{
			"help:args": "ISSUE-KEY\tIssue key of the source issue, eg: ISSUE-1\n" +
				"COMMENT_BODY\tBody of the comment you want to add",
		},
		Run: add,
	}

	cmd.Flags().Bool("web", false, "Open issue in web browser after adding comment")
	cmd.Flags().StringP("template", "T", "", "Path to a file to read comment body from")
	cmd.Flags().Bool("no-input", false, "Disable prompt for non-required fields")
	cmd.Flags().Bool("internal", false, "Make comment internal")
	cmd.Flags().String("reply-to", "", "Id of a comment to mention the author of and quote; not a threaded reply")

	return &cmd
}

func add(cmd *cobra.Command, args []string) {
	params := parseArgsAndFlags(args, cmd.Flags())
	client := api.DefaultClient(params.debug)
	ac := addCmd{
		client:    client,
		linkTypes: nil,
		params:    params,
	}

	if ac.isNonInteractive() {
		ac.params.noInput = true

		if ac.isMandatoryParamsMissing() {
			cmdutil.Failed("`ISSUE-KEY` is mandatory when using a non-interactive mode")
		}
	}

	cmdutil.ExitIfError(ac.setIssueKey())

	if ac.params.replyTo != "" {
		ac.setReply()
	}

	qs := ac.getQuestions()
	if len(qs) > 0 {
		ans := struct{ Body string }{}
		err := survey.Ask(qs, &ans)
		cmdutil.ExitIfError(err)

		params.body = ans.Body
	} else if ac.params.replyPrefix != "" {
		if ac.params.body == "" {
			cmdutil.Failed("Comment body is empty")
		}
		params.body = ac.params.replyPrefix + params.body
	}

	params.body = cmdcommon.ResolveMentions(client, params.issueKey, "", params.body, params.noInput)

	if !params.noInput {
		answer := struct{ Action string }{}
		err := survey.Ask([]*survey.Question{getNextAction()}, &answer)
		cmdutil.ExitIfError(err)

		if answer.Action == cmdcommon.ActionCancel {
			cmdutil.Failed("Action aborted")
		}
	}

	comment, err := func() (*jira.Comment, error) {
		s := cmdutil.Info("Adding comment")
		defer s.Stop()

		return client.AddIssueComment(ac.params.issueKey, ac.params.body, ac.params.internal, ac.params.visibility)
	}()
	cmdutil.ExitIfError(err)

	server := viper.GetString("server")

	cmdutil.Success("Comment added to issue %q (ID: %s)", ac.params.issueKey, comment.ID)
	fmt.Printf("%s\n", cmdutil.GenerateServerBrowseURL(server, ac.params.issueKey))

	if web, _ := cmd.Flags().GetBool("web"); web {
		err := cmdutil.Navigate(server, ac.params.issueKey)
		cmdutil.ExitIfError(err)
	}
}

type addParams struct {
	issueKey string
	body     string
	template string
	noInput  bool
	internal bool
	debug    bool

	replyTo     string
	replyPrefix string
	visibility  *jira.CommentVisibility
}

func parseArgsAndFlags(args []string, flags query.FlagParser) *addParams {
	var issueKey, body string

	nargs := len(args)
	if nargs >= 1 {
		issueKey = cmdutil.GetJiraIssueKey(viper.GetString("project.key"), args[0])
	}
	if nargs >= 2 {
		body = args[1]
	}

	debug := viper.GetBool("debug")

	template, err := flags.GetString("template")
	cmdutil.ExitIfError(err)

	noInput, err := flags.GetBool("no-input")
	cmdutil.ExitIfError(err)

	internal, err := flags.GetBool("internal")
	cmdutil.ExitIfError(err)

	replyTo, err := flags.GetString("reply-to")
	cmdutil.ExitIfError(err)

	return &addParams{
		replyTo:  replyTo,
		issueKey: issueKey,
		body:     body,
		template: template,
		noInput:  noInput,
		internal: internal,
		debug:    debug,
	}
}

type addCmd struct {
	client    *jira.Client
	linkTypes []*jira.IssueLinkType
	params    *addParams
}

func (ac *addCmd) setIssueKey() error {
	if ac.params.issueKey != "" {
		return nil
	}

	var ans string

	qs := &survey.Question{
		Name:     "issueKey",
		Prompt:   &survey.Input{Message: "Issue key"},
		Validate: survey.Required,
	}
	if err := survey.Ask([]*survey.Question{qs}, &ans); err != nil {
		return err
	}
	ac.params.issueKey = cmdutil.GetJiraIssueKey(viper.GetString("project.key"), ans)

	return nil
}

// setReply fetches the comment that is being answered and prepares the prefix of
// the reply. The reply inherits the visibility of the original, because a reply
// that is more public than what it quotes would leak it.
func (ac *addCmd) setReply() {
	original, err := func() (*jira.Comment, error) {
		s := cmdutil.Info("Fetching comment")
		defer s.Stop()

		return api.ProxyGetIssueComment(ac.client, ac.params.issueKey, ac.params.replyTo)
	}()
	cmdutil.ExitIfError(err)

	local := viper.GetString("installation") == jira.InstallationTypeLocal
	ac.params.replyPrefix = cmdcommon.ReplyPrefix(original, local)

	if original.IsInternal() && !ac.params.internal {
		ac.params.internal = true
		cmdutil.Warn("Comment %s is internal, so the reply is internal too", ac.params.replyTo)
	}
	if v := original.Visibility; v != nil {
		ac.params.visibility = v
		cmdutil.Warn("Comment %s is restricted to %s %q, so the reply is too", ac.params.replyTo, v.Type, v.Value)
	}
}

func (ac *addCmd) getQuestions() []*survey.Question {
	var (
		qs          []*survey.Question
		defaultBody string
	)

	if ac.params.template != "" || cmdutil.StdinHasData() {
		b, err := cmdutil.ReadFile(ac.params.template)
		if err != nil {
			cmdutil.Failed("Error: %s", err)
		}
		defaultBody = string(b)
	}

	if ac.params.noInput && ac.params.body == "" {
		ac.params.body = defaultBody
		return qs
	}

	if ac.params.body == "" {
		qs = append(qs, &survey.Question{
			Name: "body",
			Prompt: &surveyext.JiraEditor{
				Editor: &survey.Editor{
					Message:       "Comment body",
					Default:       ac.params.replyPrefix + defaultBody,
					HideDefault:   true,
					AppendDefault: true,
				},
				BlankAllowed: false,
			},
		})
	}

	return qs
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

func (ac *addCmd) isNonInteractive() bool {
	return cmdutil.StdinHasData() || ac.params.template == "-"
}

func (ac *addCmd) isMandatoryParamsMissing() bool {
	return ac.params.issueKey == ""
}
