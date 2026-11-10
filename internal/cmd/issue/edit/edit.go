package edit

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdcommon"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/internal/query"
	"github.com/rethab/jira-cli/pkg/adf"
	"github.com/rethab/jira-cli/pkg/jira"
	"github.com/rethab/jira-cli/pkg/surveyext"
)

const (
	helpText = `Edit an issue in a given project with minimal information.`
	examples = `$ jira issue edit ISSUE-1

# Edit issue in the configured project
$ jira issue edit ISSUE-1 -s"New Bug" -yHigh -lbug -lurgent -CBackend -b"Bug description"

# Use --no-input option to disable interactive prompt
$ jira issue edit ISSUE-1 -s"New updated summary" --no-input

# Set the original estimate; the remaining estimate is left to Jira
$ jira issue edit ISSUE-1 -e"1d 4h" --no-input

# Change the issue type; Jira refuses if the issue's status isn't in the new type's workflow,
# in which case move the issue in the Jira web UI
$ jira issue edit ISSUE-1 -t Bug --no-input

# Replacing a description that contains inline images or attachments would delete them,
# so the command refuses unless you explicitly accept the loss
$ jira issue edit ISSUE-1 -b"New description" --allow-image-loss

# Load the new description from a template file
$ jira issue edit ISSUE-1 --template /path/to/template.tmpl

# Use minus (-) to read the template from standard input
$ cat template.tmpl | jira issue edit ISSUE-1 --template -

# Append to the description without rewriting it; a leading list joins a list the description ends with
$ jira issue edit ISSUE-1 --append-body "- Runbook: https://example.com/runbook" --no-input

# Insert at the start of the description; images and other content below stay as they are
$ jira issue edit ISSUE-1 --prepend-body "**Update:** fixed in v2.0" --no-input

# Use pipe to read the description body directly from standard input
$ echo "Description from stdin" | jira issue edit ISSUE-1 -s"New updated summary"  --no-input

# Use minus (-) to remove label, component or fixVersion
$ jira issue edit ISSUE-1 --label -urgent --component -BE --fix-version -v1.0`
)

// Only offered when editing, since "create" doesn't prompt for it.
const metadataOriginalEstimate = "OriginalEstimate"

// NewCmdEdit is an edit command.
func NewCmdEdit() *cobra.Command {
	cmd := cobra.Command{
		Use:     "edit ISSUE-KEY",
		Short:   "Edit an issue in a project",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"update", "modify"},
		Annotations: map[string]string{
			"help:args": `ISSUE-KEY	Issue key, eg: ISSUE-1`,
		},
		Args: cobra.MinimumNArgs(1),
		Run:  edit,
	}

	setFlags(&cmd)

	return &cmd
}

func edit(cmd *cobra.Command, args []string) {
	server := viper.GetString("server")
	project := viper.GetString("project.key")

	params := parseArgsAndFlags(cmd.Flags(), args, project)
	params.appendBodySet = cmd.Flags().Changed("append-body")
	params.prependBodySet = cmd.Flags().Changed("prepend-body")
	cmdutil.ExitIfError(checkDescriptionAdditions(params))
	cmdutil.ExitIfError(loadTemplate(params))

	client := api.DefaultClient(params.debug)
	ec := editCmd{
		client: client,
		params: params,
	}

	customFields, err := cmdcommon.ResolveCustomFields(client, params.customFields)
	cmdutil.ExitIfError(err)

	issue, err := func() (*jira.Issue, error) {
		s := cmdutil.Info(fmt.Sprintf("Fetching issue %s...", params.issueKey))
		defer s.Stop()

		issue, err := api.ProxyGetIssue(client, params.issueKey)
		if err != nil {
			return nil, err
		}

		return issue, nil
	}()
	cmdutil.ExitIfError(err)

	originalBody := editorBody(issue)

	// Refuse a --body before the prompts rather than after the user has answered them.
	if !sameBody(params.body, originalBody) {
		cmdutil.ExitIfError(checkImageLoss(issue, params))
	}

	cmdutil.ExitIfError(ec.askQuestions(issue, originalBody))

	if !params.noInput {
		getAnswers(params, issue)
	}

	// Use stdin only if nothing is passed to --body or --template; --template - has
	// already consumed and closed it, so reading again would fail.
	if shouldReadBodyFromStdin(params, cmdutil.StdinHasData()) {
		b, err := cmdutil.ReadFile("-")
		if err != nil {
			cmdutil.Failed("Error: %s", err)
		}
		params.body = string(b)
	}
	// Keep body as is if there were no changes.
	if params.body != "" && sameBody(params.body, originalBody) {
		params.body = ""
	}

	cmdutil.ExitIfError(checkImageLoss(issue, params))

	params.body = cmdcommon.ResolveMentions(client, params.issueKey, project, params.body, params.noInput)
	params.appendBody = cmdcommon.ResolveMentions(client, params.issueKey, project, params.appendBody, params.noInput)
	params.prependBody = cmdcommon.ResolveMentions(client, params.issueKey, project, params.prependBody, params.noInput)

	descriptionOnly := onlyAddingToDescription(params)
	if !descriptionOnly {
		edr := buildEditRequest(client, project, issue, params)
		edr.WithCustomFields(customFields)

		err = func() error {
			s := cmdutil.Info("Updating an issue...")
			defer s.Stop()

			return client.Edit(params.issueKey, edr)
		}()
		if params.issueType != "" && issueTypeRejected(err) {
			cmdutil.Warn("Nothing was updated: Jira refused the issue type. Check that the type exists in the project; " +
				"if it does and the issue's status isn't in the new type's workflow, move the issue in the Jira web UI instead")
		}
		cmdutil.ExitIfError(err)
	}

	if params.prependBody != "" || params.appendBody != "" {
		local := viper.GetString("installation") == jira.InstallationTypeLocal
		err := func() error {
			s := cmdutil.Info("Updating the description...")
			defer s.Stop()

			return extendDescription(client, params.issueKey, params.prependBody, params.appendBody, local, params.skipNotify)
		}()
		if err != nil && descriptionOnly {
			cmdutil.Failed("Updating the description failed: %s", err)
		}
		if err != nil {
			cmdutil.Failed("Other changes were saved, but updating the description failed: %s", err)
		}
	}

	cmdutil.Success("Issue updated\n%s", cmdutil.GenerateServerBrowseURL(server, params.issueKey))

	handleUserAssign(project, params.issueKey, params.assignee, client)

	if web, _ := cmd.Flags().GetBool("web"); web {
		err := cmdutil.Navigate(server, params.issueKey)
		cmdutil.ExitIfError(err)
	}
}

// The editor always works with markdown, and the body coming out of it is always
// translated back to wiki markup on submit. Cloud serves the description as ADF and
// Server as wiki markup, so both have to be translated to markdown first, otherwise
// an unchanged Server description would be re-interpreted as markdown on submit.
func editorBody(issue *jira.Issue) string {
	return cmdcommon.EditorBody(issue.Fields.Description)
}

// The editor renders media as a "[attachment]" placeholder and the description is
// submitted as wiki markup, so replacing a description that embeds media makes Jira
// drop the images and delete their attachments. Only Cloud descriptions (ADF) can
// embed media.
func checkImageLoss(issue *jira.Issue, params *editParams) error {
	if params.body == "" || params.allowImageLoss || !embedsMedia(issue) {
		return nil
	}
	return fmt.Errorf(
		"the description of %s contains inline images or attachments that would be deleted if the description is replaced; "+
			"use --allow-image-loss to proceed anyway, or edit the description in the Jira web UI",
		params.issueKey,
	)
}

func embedsMedia(issue *jira.Issue) bool {
	doc, ok := issue.Fields.Description.(*adf.ADF)
	return ok && doc.HasMedia()
}

// Editors commonly append a trailing newline on save, and the markdown rendered from
// ADF doesn't end with one when the description ends in media, so trailing whitespace
// alone must not count as an edit.
func sameBody(a, b string) bool {
	return strings.TrimRight(a, " \t\r\n") == strings.TrimRight(b, " \t\r\n")
}

// The description body is always converted from markdown to Jira wiki markup since
// the edit endpoint (v2) expects wiki markup regardless of whether the original
// description was ADF (Cloud) or plain text (Server).
func buildEditRequest(client *jira.Client, project string, issue *jira.Issue, params *editParams) *jira.EditRequest {
	labels := params.labels
	labels = append(labels, issue.Fields.Labels...)

	components := make([]string, 0, len(issue.Fields.Components)+len(params.components))
	for _, c := range issue.Fields.Components {
		components = append(components, c.Name)
	}
	components = append(components, params.components...)

	fixVersions := make([]string, 0, len(issue.Fields.FixVersions)+len(params.fixVersions))
	for _, fv := range issue.Fields.FixVersions {
		fixVersions = append(fixVersions, fv.Name)
	}
	fixVersions = append(fixVersions, params.fixVersions...)

	affectsVersions := make([]string, 0, len(issue.Fields.AffectsVersions)+len(params.affectsVersions))
	for _, fv := range issue.Fields.AffectsVersions {
		affectsVersions = append(affectsVersions, fv.Name)
	}
	affectsVersions = append(affectsVersions, params.affectsVersions...)

	parent := cmdutil.GetJiraIssueKey(project, params.parentIssueKey)
	if parent == "" && issue.Fields.Parent != nil {
		parent = issue.Fields.Parent.Key
	}

	edr := &jira.EditRequest{
		IssueType:        params.issueType,
		ParentIssueKey:   parent,
		Summary:          params.summary,
		Body:             client.ToJiraMD(params.body),
		Priority:         params.priority,
		Labels:           labels,
		Components:       components,
		FixVersions:      fixVersions,
		AffectsVersions:  affectsVersions,
		OriginalEstimate: params.originalEstimate,
		CustomFields:     params.customFields,
		SkipNotify:       params.skipNotify,
	}

	return edr
}

func getAnswers(params *editParams, issue *jira.Issue) {
	answer := struct{ Action string }{}
	for answer.Action != cmdcommon.ActionSubmit {
		err := survey.Ask([]*survey.Question{cmdcommon.GetNextAction()}, &answer)
		cmdutil.ExitIfError(err)

		switch answer.Action {
		case cmdcommon.ActionCancel:
			cmdutil.Failed("Action aborted")
		case cmdcommon.ActionMetadata:
			ans := struct{ Metadata []string }{}
			err := survey.Ask(cmdcommon.GetMetadata(metadataOriginalEstimate), &ans)
			cmdutil.ExitIfError(err)

			if len(ans.Metadata) > 0 {
				qs := getMetadataQuestions(ans.Metadata, issue)
				ans := struct {
					Priority         string
					Labels           string
					Components       string
					FixVersions      string
					AffectsVersions  string
					OriginalEstimate string
				}{}
				err := survey.Ask(qs, &ans)
				cmdutil.ExitIfError(err)

				if ans.Priority != "" {
					params.priority = ans.Priority
				}
				if len(ans.Labels) > 0 {
					params.labels = strings.Split(ans.Labels, ",")
				}
				if len(ans.Components) > 0 {
					params.components = strings.Split(ans.Components, ",")
				}
				if len(ans.FixVersions) > 0 {
					params.fixVersions = strings.Split(ans.FixVersions, ",")
				}
				if len(ans.AffectsVersions) > 0 {
					params.affectsVersions = strings.Split(ans.AffectsVersions, ",")
				}
				applyOriginalEstimateAnswer(params, issue, ans.OriginalEstimate)
			}
		}
	}
}

// The answer wins over --original-estimate, like the other metadata. Confirming the
// current estimate must not resend it, and an empty answer can't clear it.
func applyOriginalEstimateAnswer(params *editParams, issue *jira.Issue, answer string) {
	switch answer {
	case "":
	case issue.Fields.TimeTracking.OriginalEstimate:
		params.originalEstimate = ""
	default:
		params.originalEstimate = answer
	}
}

func handleUserAssign(project, key, assignee string, client *jira.Client) {
	if assignee == "" {
		return
	}
	if strings.EqualFold(assignee, cmdcommon.AssigneeUnassign) {
		if err := api.ProxyAssignIssue(client, key, nil, jira.AssigneeNone); err != nil {
			cmdutil.Failed("Unable to unassign user: %s", err.Error())
		}
		return
	}
	if strings.EqualFold(assignee, jira.AssigneeDefault) {
		if err := api.ProxyAssignIssue(client, key, nil, jira.AssigneeDefault); err != nil {
			cmdutil.Failed("Unable to set default assignee: %s", err.Error())
		}
		return
	}
	user, err := api.ProxyUserSearch(client, &jira.UserSearchOptions{
		Query:    assignee,
		Project:  project,
		IssueKey: key,
	})
	if err != nil || len(user) == 0 {
		cmdutil.Failed("Unable to find assignee")
	}
	if err = api.ProxyAssignIssue(client, key, user[0], assignee); err != nil {
		cmdutil.Failed("Unable to set assignee: %s", err.Error())
	}
}

type editCmd struct {
	client *jira.Client
	params *editParams
}

func (ec *editCmd) askQuestions(issue *jira.Issue, originalBody string) error {
	if ec.params.noInput {
		return nil
	}

	ans := struct{ Summary, Body string }{}
	err := survey.Ask(ec.questions(issue, originalBody), &ans)
	if err != nil {
		return err
	}

	if ec.params.summary == "" {
		ec.params.summary = ans.Summary
	}
	if ec.params.body == "" {
		ec.params.body = ans.Body
	}

	return nil
}

func (ec *editCmd) questions(issue *jira.Issue, originalBody string) []*survey.Question {
	var qs []*survey.Question

	if ec.params.summary == "" {
		qs = append(qs, &survey.Question{
			Name: "summary",
			Prompt: &survey.Input{
				Message: "Summary",
				Default: issue.Fields.Summary,
			},
			Validate: survey.Required,
		})
	}

	if ec.params.body != "" || ec.params.appendBody != "" || ec.params.prependBody != "" {
		return qs
	}
	// Any change made in the editor would be refused by checkImageLoss, discarding
	// what the user typed, so don't offer the editor in the first place.
	if !ec.params.allowImageLoss && embedsMedia(issue) {
		cmdutil.Warn("Skipping description: it contains inline images or attachments that would be deleted " +
			"if it is replaced; use --allow-image-loss to edit it anyway")
		return qs
	}
	return append(qs, &survey.Question{
		Name: "body",
		Prompt: &surveyext.JiraEditor{
			Editor: &survey.Editor{
				Message:       "Description",
				Default:       originalBody,
				HideDefault:   true,
				AppendDefault: true,
			},
			BlankAllowed: true,
		},
	})
}

// Jira changes the type in place only if the issue's status exists in the new type's
// workflow, and its error doesn't mention that moving the issue in the web UI is the way out.
func issueTypeRejected(err error) bool {
	var unexpected *jira.ErrUnexpectedResponse
	return errors.As(err, &unexpected) && unexpected.Body.Errors["issuetype"] != ""
}

type editParams struct {
	issueKey         string
	issueType        string
	parentIssueKey   string
	summary          string
	body             string
	template         string
	appendBody       string
	appendBodySet    bool
	prependBody      string
	prependBodySet   bool
	priority         string
	assignee         string
	labels           []string
	components       []string
	fixVersions      []string
	affectsVersions  []string
	originalEstimate string
	customFields     map[string]string
	skipNotify       bool
	allowImageLoss   bool
	noInput          bool
	debug            bool
}

// An explicit --body wins, and the template is resolved before the issue is fetched
// so that checkImageLoss and the prompts see the body it provides.
func loadTemplate(params *editParams) error {
	if params.body != "" || params.template == "" {
		return nil
	}
	b, err := cmdutil.ReadFile(params.template)
	if err != nil {
		return fmt.Errorf("unable to read template: %w", err)
	}
	params.body = string(b)
	return nil
}

func parseArgsAndFlags(flags query.FlagParser, args []string, project string) *editParams {
	issueType, err := flags.GetString("type")
	cmdutil.ExitIfError(err)

	parentIssueKey, err := flags.GetString("parent")
	cmdutil.ExitIfError(err)

	summary, err := flags.GetString("summary")
	cmdutil.ExitIfError(err)

	body, err := flags.GetString("body")
	cmdutil.ExitIfError(err)

	template, err := flags.GetString("template")
	cmdutil.ExitIfError(err)

	appendBody, err := flags.GetString("append-body")
	cmdutil.ExitIfError(err)

	prependBody, err := flags.GetString("prepend-body")
	cmdutil.ExitIfError(err)

	priority, err := flags.GetString("priority")
	cmdutil.ExitIfError(err)

	assignee, err := flags.GetString("assignee")
	cmdutil.ExitIfError(err)

	labels, err := flags.GetStringArray("label")
	cmdutil.ExitIfError(err)

	components, err := flags.GetStringArray("component")
	cmdutil.ExitIfError(err)

	fixVersions, err := flags.GetStringArray("fix-version")
	cmdutil.ExitIfError(err)

	affectsVersions, err := flags.GetStringArray("affects-version")
	cmdutil.ExitIfError(err)

	originalEstimate, err := flags.GetString("original-estimate")
	cmdutil.ExitIfError(err)

	custom, err := flags.GetStringToString("custom")
	cmdutil.ExitIfError(err)

	skipNotify, err := flags.GetBool("skip-notify")
	cmdutil.ExitIfError(err)

	allowImageLoss, err := flags.GetBool("allow-image-loss")
	cmdutil.ExitIfError(err)

	noInput, err := flags.GetBool("no-input")
	cmdutil.ExitIfError(err)

	// Reading the template consumes stdin, so there is nothing left to prompt on.
	// An explicit --body wins over the template, in which case stdin is left alone.
	if template == "-" && body == "" {
		noInput = true
	}

	debug := viper.GetBool("debug")

	return &editParams{
		issueKey:         cmdutil.GetJiraIssueKey(project, args[0]),
		issueType:        issueType,
		parentIssueKey:   parentIssueKey,
		summary:          summary,
		body:             body,
		template:         template,
		appendBody:       appendBody,
		prependBody:      prependBody,
		priority:         priority,
		assignee:         assignee,
		labels:           labels,
		components:       components,
		fixVersions:      fixVersions,
		affectsVersions:  affectsVersions,
		originalEstimate: originalEstimate,
		customFields:     custom,
		skipNotify:       skipNotify,
		allowImageLoss:   allowImageLoss,
		noInput:          noInput,
		debug:            debug,
	}
}

func getMetadataQuestions(meta []string, issue *jira.Issue) []*survey.Question {
	var qs []*survey.Question

	fixVersions := make([]string, 0, len(issue.Fields.FixVersions))
	for _, fv := range issue.Fields.FixVersions {
		fixVersions = append(fixVersions, fv.Name)
	}

	affectsVersions := make([]string, 0, len(issue.Fields.AffectsVersions))
	for _, fv := range issue.Fields.AffectsVersions {
		affectsVersions = append(affectsVersions, fv.Name)
	}

	for _, m := range meta {
		switch m {
		case "Priority":
			qs = append(qs, &survey.Question{
				Name:   "priority",
				Prompt: &survey.Input{Message: "Priority", Default: issue.Fields.Priority.Name},
			})
		case "Components":
			qs = append(qs, &survey.Question{
				Name: "components",
				Prompt: &survey.Input{
					Message: "Components",
					Help:    "Comma separated list of valid components. For eg: BE,FE",
				},
			})
		case "Labels":
			qs = append(qs, &survey.Question{
				Name: "labels",
				Prompt: &survey.Input{
					Message: "Labels",
					Help:    "Comma separated list of labels. For eg: backend,urgent",
					Default: strings.Join(issue.Fields.Labels, ","),
				},
			})
		case "FixVersions":
			qs = append(qs, &survey.Question{
				Name: "fixversions",
				Prompt: &survey.Input{
					Message: "Fix Versions",
					Help:    "Comma separated list of fixVersions. For eg: v1.0-beta,v2.0",
					Default: strings.Join(fixVersions, ","),
				},
			})
		case metadataOriginalEstimate:
			qs = append(qs, &survey.Question{
				Name: "originalestimate",
				Prompt: &survey.Input{
					Message: "Original estimate",
					Help:    cmdcommon.OriginalEstimateHelp,
					Default: issue.Fields.TimeTracking.OriginalEstimate,
				},
			})
		case "AffectsVersions":
			qs = append(qs, &survey.Question{
				Name: "affectsversions",
				Prompt: &survey.Input{
					Message: "Affects Versions",
					Help:    "Comma separated list of affectsVersions. For eg: v1.0-beta,v2.0",
					Default: strings.Join(affectsVersions, ","),
				},
			})
		}
	}

	return qs
}

func setFlags(cmd *cobra.Command) {
	custom := make(map[string]string)

	cmd.Flags().SortFlags = false

	cmd.Flags().StringP("type", "t", "", "Change issue type (the issue's status must exist in the new type's workflow)")
	cmd.Flags().StringP("parent", "P", "", `Link to a parent key`)
	cmd.Flags().StringP("summary", "s", "", "Edit summary or title")
	cmd.Flags().StringP("body", "b", "", "Edit description")
	cmd.Flags().StringP("template", "T", "", "Path to a file to read the description from, or - for standard input")
	cmd.Flags().String("append-body", "", "Append Markdown to the end of the description, leaving the existing content untouched")
	cmd.Flags().String("prepend-body", "", "Insert Markdown at the start of the description, leaving the existing content untouched")
	cmd.Flags().StringP("priority", "y", "", "Edit priority")
	cmd.Flags().StringP("assignee", "a", "", "Edit assignee (email or display name)")
	cmd.Flags().StringArrayP("label", "l", []string{}, "Append labels")
	cmd.Flags().StringArrayP("component", "C", []string{}, "Replace components")
	cmd.Flags().StringArray("fix-version", []string{}, "Add/Append release info (fixVersions)")
	cmd.Flags().StringArray("affects-version", []string{}, "Add/Append release info (affectsVersions)")
	cmd.Flags().StringP("original-estimate", "e", "", cmdcommon.OriginalEstimateHelp)
	cmd.Flags().StringToString("custom", custom, "Edit custom fields")
	cmd.Flags().Bool("skip-notify", false, "Do not notify watchers about the issue update")
	cmd.Flags().Bool("allow-image-loss", false, "Replace a description even if its inline images and attachments get deleted")
	cmd.Flags().Bool("web", false, "Open in web browser after successful update")
	cmd.Flags().Bool("no-input", false, "Disable prompt for non-required fields")
}
