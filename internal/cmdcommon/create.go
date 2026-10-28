package cmdcommon

import (
	"fmt"
	"slices"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	// ActionSubmit is a submit action.
	ActionSubmit = "Submit"
	// ActionCancel is a cancel action.
	ActionCancel = "Cancel"
	// ActionMetadata is an add metadata action.
	ActionMetadata = "Add metadata"
	// AssigneeUnassign is the shorthand used to unassign an issue.
	AssigneeUnassign = "x"
)

// CreateParams holds parameters for create command.
type CreateParams struct {
	Name             string
	IssueType        string
	ParentIssueKey   string
	Summary          string
	Body             string
	Priority         string
	Reporter         string
	Assignee         string
	Labels           []string
	Components       []string
	FixVersions      []string
	AffectsVersions  []string
	OriginalEstimate string
	CustomFields     map[string]string
	Template         string
	NoInput          bool
	Debug            bool
}

// SetCreateFlags sets flags supported by create command.
func SetCreateFlags(cmd *cobra.Command, prefix string) {
	custom := make(map[string]string)

	cmd.Flags().SortFlags = false

	if prefix == "Epic" {
		cmd.Flags().StringP("name", "n", "", "Epic name")
	} else {
		cmd.Flags().StringP("type", "t", "", "Issue type")
		cmd.Flags().StringP("parent", "P", "", `Parent issue key can be used to attach epic to an issue.
And, this field is mandatory when creating a sub-task.`)
	}
	cmd.Flags().StringP("summary", "s", "", prefix+" summary or title")
	cmd.Flags().StringP("body", "b", "", prefix+" description")
	cmd.Flags().StringP("priority", "y", "", prefix+" priority")
	cmd.Flags().StringP("reporter", "r", "", prefix+" reporter (username, email or display name)")
	cmd.Flags().StringP("assignee", "a", "", prefix+" assignee (username, email or display name)")
	cmd.Flags().StringArrayP("label", "l", []string{}, prefix+" labels")
	cmd.Flags().StringArrayP("component", "C", []string{}, prefix+" components")
	cmd.Flags().StringArray("fix-version", []string{}, "Release info (fixVersions)")
	cmd.Flags().StringArray("affects-version", []string{}, "Release info (affectsVersions)")
	cmd.Flags().StringP("original-estimate", "e", "", OriginalEstimateHelp)
	cmd.Flags().StringToString("custom", custom, "Set custom fields")
	cmd.Flags().StringP("template", "T", "", "Path to a file to read body/description from")
	cmd.Flags().Bool("web", false, "Open in web browser after successful creation")
	cmd.Flags().Bool("no-input", false, "Disable prompt for non-required fields")
}

// OriginalEstimateHelp is the help text of the original estimate flag.
const OriginalEstimateHelp = "Original estimate (1w 2d 3h 4m: weeks, days, hours, minutes)"

// GetNextAction provide user an option to select next action.
func GetNextAction() *survey.Question {
	return &survey.Question{
		Name: "action",
		Prompt: &survey.Select{
			Message: "What's next?",
			Options: []string{
				ActionSubmit,
				ActionMetadata,
				ActionCancel,
			},
		},
		Validate: survey.Required,
	}
}

// GetMetadata gathers a list of metadata users wants to add. Commands that support more
// than the common options pass them as extra.
func GetMetadata(extra ...string) []*survey.Question {
	return []*survey.Question{
		{
			Name: "metadata",
			Prompt: &survey.MultiSelect{
				Message: "What would you like to add?",
				Options: append([]string{"Priority", "Components", "Labels", "FixVersions", "AffectsVersions"}, extra...),
			},
		},
	}
}

// GetMetadataQuestions prepares metadata question to input from user.
func GetMetadataQuestions(cat []string) []*survey.Question {
	var qs []*survey.Question

	for _, c := range cat {
		switch c {
		case "Priority":
			qs = append(qs, &survey.Question{
				Name:   "priority",
				Prompt: &survey.Input{Message: "Priority"},
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
				},
			})
		case "FixVersions":
			qs = append(qs, &survey.Question{
				Name: "fixversions",
				Prompt: &survey.Input{
					Message: "Fix Versions",
					Help:    "Comma separated list of fixVersions. For eg: v1.0-beta,v2.0",
				},
			})
		case "AffectsVersions":
			qs = append(qs, &survey.Question{
				Name: "affectsversions",
				Prompt: &survey.Input{
					Message: "Affects Versions",
					Help:    "Comma separated list of affectsVersions. For eg: v1.0-beta,v2.0",
				},
			})
		}
	}

	return qs
}

// HandleNoInput handles operations for --no-input flag.
func HandleNoInput(params *CreateParams) error {
	answer := struct{ Action string }{}
	for answer.Action != ActionSubmit {
		err := survey.Ask([]*survey.Question{GetNextAction()}, &answer)
		if err != nil {
			return err
		}

		switch answer.Action {
		case ActionCancel:
			cmdutil.Failed("Action aborted")
		case ActionMetadata:
			ans := struct{ Metadata []string }{}
			err := survey.Ask(GetMetadata(), &ans)
			if err != nil {
				return err
			}

			if len(ans.Metadata) > 0 {
				qs := GetMetadataQuestions(ans.Metadata)
				ans := struct {
					Priority        string
					Labels          string
					Components      string
					FixVersions     string
					AffectsVersions string
				}{}
				err := survey.Ask(qs, &ans)
				if err != nil {
					return err
				}

				if ans.Priority != "" {
					params.Priority = ans.Priority
				}
				if len(ans.Labels) > 0 {
					params.Labels = strings.Split(ans.Labels, ",")
				}
				if len(ans.Components) > 0 {
					params.Components = strings.Split(ans.Components, ",")
				}
				if len(ans.FixVersions) > 0 {
					params.FixVersions = strings.Split(ans.FixVersions, ",")
				}
				if len(ans.AffectsVersions) > 0 {
					params.AffectsVersions = strings.Split(ans.AffectsVersions, ",")
				}
			}
		}
	}
	return nil
}

// GetRelevantUser finds and returns a valid user name or account ID based on user input.
func GetRelevantUser(client *jira.Client, project string, user string) string {
	if user == "" {
		return ""
	}
	u, err := api.ProxyUserSearch(client, &jira.UserSearchOptions{
		Query:   user,
		Project: project,
	})
	if err != nil || len(u) == 0 {
		cmdutil.Failed("Unable to find associated user for %s", user)
	}
	return GetUserKeyForConfiguredInstallation(u[0])
}

// GetRelevantAssignee finds and returns a valid assignee based on user input. The "x" and
// "default" options are sentinels rather than user names, so they are handed over to the API
// layer as-is instead of being looked up as a user.
func GetRelevantAssignee(client *jira.Client, project string, assignee string) string {
	switch {
	case strings.EqualFold(assignee, AssigneeUnassign):
		return jira.AssigneeNone
	case strings.EqualFold(assignee, jira.AssigneeDefault):
		return jira.AssigneeDefault
	}
	return GetRelevantUser(client, project, assignee)
}

// GetUserKeyForConfiguredInstallation returns either the user name or account ID based on jira installation type.
func GetUserKeyForConfiguredInstallation(user *jira.User) string {
	it := viper.GetString("installation")
	if it == jira.InstallationTypeLocal {
		return user.Name
	}
	return user.AccountID
}

// GetConfiguredCustomFields returns the custom fields configured by the user.
func GetConfiguredCustomFields() ([]jira.IssueTypeField, error) {
	var configuredFields []jira.IssueTypeField

	err := viper.UnmarshalKey("issue.fields.custom", &configuredFields)
	if err != nil {
		return nil, err
	}

	return configuredFields, nil
}

// ResolveCustomFields returns the fields the keys passed to `--custom` refer to. Keys are
// matched against the configured fields first; only if some match nothing, the fields are
// looked up on the server once, which covers fields created after `jira init`. Keys that
// still match nothing are an error, since carrying on would silently drop them.
//
// Call this before prompting or showing a spinner, so a bad key fails the command up front.
func ResolveCustomFields(client *jira.Client, fields map[string]string) ([]jira.IssueTypeField, error) {
	// Ignoring a malformed config keeps the lookup path available for the keys.
	configured, _ := GetConfiguredCustomFields()

	unmatched := unmatchedKeys(fields, configured)
	if len(unmatched) == 0 {
		return configured, nil
	}

	all, err := client.GetField()
	if err != nil {
		return nil, fmt.Errorf("looking up custom fields %s: %w", strings.Join(unmatched, ", "), err)
	}

	for _, f := range all {
		if !f.Custom {
			continue
		}

		looked := jira.IssueTypeField{Name: f.Name, Key: f.ID}
		looked.Schema.DataType = f.Schema.DataType
		looked.Schema.Items = f.Schema.Items

		if slices.ContainsFunc(unmatched, func(k string) bool { return jira.CustomFieldMatches(k, looked) }) {
			configured = append(configured, looked)
		}
	}

	if unknown := unmatchedKeys(fields, configured); len(unknown) > 0 {
		return nil, fmt.Errorf(
			"unknown custom fields: %s\n"+
				"Run `jira issue fields -t TYPE` to list the fields of an issue type, "+
				"or pass the field id as `customfield_NNNNN`",
			strings.Join(unknown, ", "),
		)
	}

	return configured, nil
}

func unmatchedKeys(fields map[string]string, configured []jira.IssueTypeField) []string {
	var out []string

	for key := range fields {
		if !slices.ContainsFunc(configured, func(c jira.IssueTypeField) bool { return jira.CustomFieldMatches(key, c) }) {
			out = append(out, key)
		}
	}
	slices.Sort(out)

	return out
}
