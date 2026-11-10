package list

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `List shows the remote web links of an issue, including the ones Confluence adds when a page mentions it.

The URL is what "jira issue unlink remote" takes to remove a link.`
	examples = `$ jira issue link remote list ISSUE-1

# Tab separated id, title and URL, for scripts
$ jira issue link remote list ISSUE-1 --plain`
)

// Titles are free text and would break one line per link.
var flatten = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ")

// NewCmdRemoteLinkList is a remote link list command.
func NewCmdRemoteLinkList() *cobra.Command {
	cmd := cobra.Command{
		Use:     "list ISSUE_KEY",
		Short:   "List the remote web links of an issue",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"ls"},
		Annotations: map[string]string{
			"help:args": "ISSUE_KEY\tIssue key, eg: ISSUE-1",
		},
		Args: cobra.ExactArgs(1),
		Run:  list,
	}

	cmd.Flags().Bool("plain", false, "Print tab separated id, title and URL")

	return &cmd
}

func list(cmd *cobra.Command, args []string) {
	plain, err := cmd.Flags().GetBool("plain")
	cmdutil.ExitIfError(err)

	key := cmdutil.GetJiraIssueKey(viper.GetString("project.key"), args[0])
	client := api.DefaultClient(viper.GetBool("debug"))

	links, err := func() ([]*jira.RemoteLink, error) {
		s := cmdutil.Info("Fetching remote links...")
		defer s.Stop()

		return client.GetRemoteLinks(key)
	}()
	cmdutil.ExitIfError(err)

	if out := format(links, plain); out != "" {
		fmt.Println(out)
	}
}

func format(links []*jira.RemoteLink, plain bool) string {
	if len(links) == 0 {
		return ""
	}

	lines := make([]string, 0, len(links)+1)
	if !plain {
		lines = append(lines, "ID\tTITLE\tURL")
	}
	for _, l := range links {
		lines = append(lines, strings.Join([]string{strconv.Itoa(l.ID), flatten.Replace(l.Object.Title), l.Object.URL}, "\t"))
	}

	// tabwriter would pad the columns, and scripts need the bare tabs.
	if plain {
		return strings.Join(lines, "\n")
	}

	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, l := range lines {
		_, _ = fmt.Fprintln(w, l)
	}
	_ = w.Flush()

	return strings.TrimSuffix(b.String(), "\n")
}
