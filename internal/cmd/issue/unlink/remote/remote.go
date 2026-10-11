package remote

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `Removes a remote web link from an issue. The link to remove is identified by its URL.`
	examples = `$ jira issue unlink remote ISSUE-1 http://weblink.com`
)

// NewCmdRemoteUnlink is a remote unlink command.
func NewCmdRemoteUnlink() *cobra.Command {
	cmd := cobra.Command{
		Use:     "remote ISSUE_KEY WEBLINK_URL",
		Short:   "Removes a remote web link from an issue",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"rmuln"},
		Annotations: map[string]string{
			"help:args": "ISSUE_KEY\tIssue key, eg: ISSUE-1\n" +
				"WEBLINK_URL\tUrl of the weblink to remove",
		},
		Run: remoteunlink,
	}

	return &cmd
}

func remoteunlink(cmd *cobra.Command, args []string) {
	project := viper.GetString("project.key")
	params := parseArgsAndFlags(args, project)
	client := api.DefaultClient(params.debug)
	uc := unlinkCmd{
		client: client,
		params: params,
	}

	cmdutil.ExitIfError(uc.setIssueKey(project))
	cmdutil.ExitIfError(uc.setRemoteLinkURL())

	err := func() error {
		s := cmdutil.Info("Removing remote web link from issue")
		defer s.Stop()

		linkID, err := client.GetRemoteLinkID(uc.params.issueKey, uc.params.url)
		if err != nil {
			if errors.Is(err, jira.ErrNoRemoteLink) {
				return fmt.Errorf("issue %s has no remote link with URL %q", uc.params.issueKey, uc.params.url)
			}
			return err
		}

		return client.DeleteRemoteLink(uc.params.issueKey, linkID)
	}()
	cmdutil.ExitIfError(err)

	server := viper.GetString("server")

	cmdutil.Success("Remote web link removed from Issue %s", uc.params.issueKey)
	fmt.Printf("%s\n", cmdutil.GenerateServerBrowseURL(server, uc.params.issueKey))

	if web, _ := cmd.Flags().GetBool("web"); web {
		err := cmdutil.Navigate(server, uc.params.issueKey)
		cmdutil.ExitIfError(err)
	}
}

type unlinkParams struct {
	issueKey string
	url      string
	debug    bool
}

func parseArgsAndFlags(args []string, project string) *unlinkParams {
	var issueKey, url string

	nargs := len(args)
	if nargs >= 1 {
		issueKey = cmdutil.GetJiraIssueKey(project, args[0])
	}
	if nargs >= 2 {
		url = args[1]
	}

	debug := viper.GetBool("debug")

	return &unlinkParams{
		issueKey: issueKey,
		url:      url,
		debug:    debug,
	}
}

type unlinkCmd struct {
	client *jira.Client
	params *unlinkParams
}

func (uc *unlinkCmd) setIssueKey(project string) error {
	if uc.params.issueKey != "" {
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
	uc.params.issueKey = cmdutil.GetJiraIssueKey(project, ans)

	return nil
}

func (uc *unlinkCmd) setRemoteLinkURL() error {
	if uc.params.url != "" {
		return nil
	}

	var ans string

	qs := &survey.Question{
		Name:     "remoteLinkURL",
		Prompt:   &survey.Input{Message: "Remote link URL"},
		Validate: survey.Required,
	}
	if err := survey.Ask([]*survey.Question{qs}, &ans); err != nil {
		return err
	}
	uc.params.url = ans

	return nil
}
