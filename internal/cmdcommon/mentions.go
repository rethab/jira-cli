package cmdcommon

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/internal/mention"
	"github.com/rethab/jira-cli/pkg/jira"
)

const mentionSearchLimit = 20

// ResolveMentions turns the @[Name] references in body into Jira mentions and
// exits if one can't be resolved, so that nothing is sent with a half-resolved
// body. Users are looked up among those who can see the issue, or the project
// when there is no issue yet. With noInput, an ambiguous reference is an error
// instead of a prompt.
func ResolveMentions(client *jira.Client, issueKey, project, body string, noInput bool) string {
	if !mention.Has(body) {
		return body
	}

	lookup := func(query string) ([]*jira.User, error) {
		s := cmdutil.Info(fmt.Sprintf("Looking up %s", query))
		defer s.Stop()

		opts := &jira.UserSearchOptions{Query: query, MaxResults: mentionSearchLimit}
		if issueKey != "" {
			opts.IssueKey = issueKey
		} else {
			opts.Project = project
		}
		return api.ProxyUserSearchViewIssue(client, opts)
	}

	var pick mention.Pick
	if !noInput {
		pick = pickMention
	}

	local := viper.GetString("installation") == jira.InstallationTypeLocal

	out, err := mention.Resolve(body, local, lookup, pick)
	cmdutil.ExitIfError(err)

	return out
}

func pickMention(query string, candidates []*jira.User) (*jira.User, error) {
	labels := make([]string, 0, len(candidates))
	seen := make(map[string]int)
	for _, u := range candidates {
		label := u.DisplayName
		if u.Email != "" {
			label += " <" + u.Email + ">"
		}
		labels = append(labels, label)
		seen[label]++
	}

	// Two users can share a name when emails are hidden, and the prompt would
	// show the same line twice.
	options := make([]string, 0, len(candidates))
	for i, u := range candidates {
		label := labels[i]
		if seen[label] > 1 {
			id := u.AccountID
			if id == "" {
				id = u.Name
			}
			label += " (" + id + ")"
		}
		options = append(options, label)
	}

	var idx int
	err := survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Who is @[%s]?", query),
		Options: options,
	}, &idx)
	if err != nil {
		return nil, err
	}
	return candidates[idx], nil
}
