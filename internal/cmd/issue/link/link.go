package link

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmd/issue/link/remote"
	"github.com/rethab/jira-cli/internal/cmd/issue/link/types"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `Link connects two issues to a given link type.

The first issue gets the outward wording of the link type and the second the inward one: "jira issue link A B Blocks" means A blocks B, and B is blocked by A. Run "jira issue link types" to list the type names and their wording.`
	examples = `# ISSUE-1 blocks ISSUE-2
$ jira issue link ISSUE-1 ISSUE-2 Blocks

# ISSUE-1 duplicates ISSUE-2
$ jira issue link ISSUE-1 ISSUE-2 Duplicate

# List the link types and their wording
$ jira issue link types`
	optionCancel = "Cancel"
)

// NewCmdLink is a link command.
func NewCmdLink() *cobra.Command {
	cmd := cobra.Command{
		Use:     "link INWARD_ISSUE_KEY OUTWARD_ISSUE_KEY ISSUE_LINK_TYPE",
		Short:   "Link connects two issues",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"ln"},
		Annotations: map[string]string{
			"help:args": "INWARD_ISSUE_KEY\tIssue that gets the outward wording of the link type, eg: ISSUE-1 in \"ISSUE-1 blocks ISSUE-2\"\n" +
				"OUTWARD_ISSUE_KEY\tIssue that gets the inward wording, eg: ISSUE-2 in \"ISSUE-2 is blocked by ISSUE-1\"\n" +
				"ISSUE_LINK_TYPE\tName of the link type, eg: Blocks, Duplicate, Relates. See \"jira issue link types\"",
		},
		Run: link,
	}

	cmd.AddCommand(remote.NewCmdRemoteLink())
	cmd.AddCommand(types.NewCmdTypes())
	cmd.PersistentFlags().Bool("web", false, "Open issue in web browser after successful linking")

	return &cmd
}

func link(cmd *cobra.Command, args []string) {
	project := viper.GetString("project.key")
	params := parseArgsAndFlags(args, project)
	client := api.DefaultClient(params.debug)
	lc := linkCmd{
		client:    client,
		linkTypes: nil,
		params:    params,
	}

	cmdutil.ExitIfError(lc.setInwardIssueKey(project))
	cmdutil.ExitIfError(lc.setOutwardIssueKey(project))
	cmdutil.ExitIfError(lc.setLinkTypes())
	cmdutil.ExitIfError(lc.setDesiredLinkType())

	if lc.params.linkType == optionCancel {
		cmdutil.Fail("Action aborted")
		os.Exit(0)
	}

	lt, err := lc.verifyIssueLinkType()
	if err != nil {
		fmt.Println()
		cmdutil.Failed("Error: %s", err.Error())
		return
	}

	err = func() error {
		s := cmdutil.Info("Linking issues")
		defer s.Stop()

		return client.LinkIssue(lc.params.inwardIssueKey, lc.params.outwardIssueKey, lt.Name)
	}()
	cmdutil.ExitIfError(err)

	server := viper.GetString("server")

	cmdutil.Success("%s", linkedMessage(lc.params.inwardIssueKey, lc.params.outwardIssueKey, lt))
	fmt.Printf("%s\n", cmdutil.GenerateServerBrowseURL(server, lc.params.inwardIssueKey))

	if web, _ := cmd.Flags().GetBool("web"); web {
		err := cmdutil.Navigate(server, lc.params.inwardIssueKey)
		cmdutil.ExitIfError(err)
	}
}

// linkTypeOption spells out the link with the chosen keys, so the prompt shows which way round it goes.
// The name comes first and is followed by a colon, which setDesiredLinkType relies on to get it back.
func linkTypeOption(inwardKey, outwardKey string, t *jira.IssueLinkType) string {
	return fmt.Sprintf("%s: %s %s %s", t.Name, inwardKey, t.Outward, outwardKey)
}

func linkedMessage(inwardKey, outwardKey string, t *jira.IssueLinkType) string {
	return fmt.Sprintf("Linked: %s %s %s", inwardKey, t.Outward, outwardKey)
}

type linkParams struct {
	inwardIssueKey  string
	outwardIssueKey string
	linkType        string
	debug           bool
}

func parseArgsAndFlags(args []string, project string) *linkParams {
	var inwardIssueKey, outwardIssueKey, linkType string

	nargs := len(args)
	if nargs >= 1 {
		inwardIssueKey = cmdutil.GetJiraIssueKey(project, args[0])
	}
	if nargs >= 2 {
		outwardIssueKey = cmdutil.GetJiraIssueKey(project, args[1])
	}
	if nargs >= 3 {
		linkType = args[2]
	}

	debug := viper.GetBool("debug")

	return &linkParams{
		inwardIssueKey:  inwardIssueKey,
		outwardIssueKey: outwardIssueKey,
		linkType:        linkType,
		debug:           debug,
	}
}

type linkCmd struct {
	client    *jira.Client
	linkTypes []*jira.IssueLinkType
	params    *linkParams
}

func (lc *linkCmd) setInwardIssueKey(project string) error {
	if lc.params.inwardIssueKey != "" {
		return nil
	}

	var ans string

	qs := &survey.Question{
		Name:     "inwardIssueKey",
		Prompt:   &survey.Input{Message: "First issue key"},
		Validate: survey.Required,
	}
	if err := survey.Ask([]*survey.Question{qs}, &ans); err != nil {
		return err
	}
	lc.params.inwardIssueKey = cmdutil.GetJiraIssueKey(project, ans)

	return nil
}

func (lc *linkCmd) setOutwardIssueKey(project string) error {
	if lc.params.outwardIssueKey != "" {
		return nil
	}

	var ans string

	qs := &survey.Question{
		Name:     "outwardIssueKey",
		Prompt:   &survey.Input{Message: "Second issue key"},
		Validate: survey.Required,
	}
	if err := survey.Ask([]*survey.Question{qs}, &ans); err != nil {
		return err
	}
	lc.params.outwardIssueKey = cmdutil.GetJiraIssueKey(project, ans)

	return nil
}

func (lc *linkCmd) setLinkTypes() error {
	s := cmdutil.Info("Fetching link types. Please wait...")
	defer s.Stop()

	lt, err := lc.client.GetIssueLinkTypes()
	if err != nil {
		return err
	}
	lc.linkTypes = lt

	return nil
}

func (lc *linkCmd) setDesiredLinkType() error {
	if lc.params.linkType != "" {
		return nil
	}

	options := make([]string, 0, len(lc.linkTypes))
	for _, t := range lc.linkTypes {
		options = append(options, linkTypeOption(lc.params.inwardIssueKey, lc.params.outwardIssueKey, t))
	}
	options = append(options, optionCancel)

	var ans string

	qs := &survey.Question{
		Name: "linkType",
		Prompt: &survey.Select{
			Message: "Link type:",
			Options: options,
		},
		Validate: survey.Required,
	}
	if err := survey.Ask([]*survey.Question{qs}, &ans); err != nil {
		return err
	}
	lc.params.linkType = strings.Split(ans, ":")[0]

	return nil
}

func (lc *linkCmd) verifyIssueLinkType() (*jira.IssueLinkType, error) {
	var lt *jira.IssueLinkType

	st := strings.ToLower(lc.params.linkType)
	all := make([]string, 0, len(lc.linkTypes))
	for _, t := range lc.linkTypes {
		if strings.ToLower(t.Name) == st {
			lt = t
		}
		all = append(all, fmt.Sprintf("'%s'", t.Name))
	}

	if lt == nil {
		return nil, fmt.Errorf(
			"invalid issue link type %q\nAvailable issue link types are: %s",
			lc.params.linkType, strings.Join(all, ", "),
		)
	}
	return lt, nil
}
