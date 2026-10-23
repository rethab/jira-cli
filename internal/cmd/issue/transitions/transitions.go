package transitions

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `Transitions lists the transitions an issue can take from its current state, without moving it.

Each line shows the transition name and the status it leads to, as in "Start work -> In Progress".`
	examples = `$ jira issue transitions ISSUE-1

# Tab separated "transition name" and "target status", for scripts
$ jira issue transitions ISSUE-1 --plain

# Print the raw Jira API response, including transitions that are not available
$ jira issue transitions ISSUE-1 --raw`
)

// NewCmdTransitions is a transitions command.
func NewCmdTransitions() *cobra.Command {
	cmd := cobra.Command{
		Use:     "transitions ISSUE-KEY",
		Short:   "List the transitions available for an issue",
		Long:    helpText,
		Example: examples,
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"help:args": `ISSUE-KEY	Issue key, eg: ISSUE-1`,
		},
		Run: transitions,
	}

	cmd.Flags().SortFlags = false

	cmd.Flags().Bool("plain", false, "Print tab separated transition name and target status")
	cmd.Flags().Bool("raw", false, "Print raw Jira API response")

	return &cmd
}

func transitions(cmd *cobra.Command, args []string) {
	project := viper.GetString("project.key")
	installation := viper.GetString("installation")
	key := cmdutil.GetJiraIssueKey(project, args[0])

	plain, err := cmd.Flags().GetBool("plain")
	cmdutil.ExitIfError(err)

	raw, err := cmd.Flags().GetBool("raw")
	cmdutil.ExitIfError(err)

	client := api.DefaultClient(viper.GetBool("debug"))

	if raw {
		// Verbatim like "issue view --raw", so unavailable transitions stay in, marked by "isAvailable".
		resp, err := func() (string, error) {
			s := cmdutil.Info("Fetching available transitions...")
			defer s.Stop()

			return api.ProxyTransitionsRaw(client, key)
		}()
		cmdutil.ExitIfError(err)

		fmt.Println(resp)
		return
	}

	all, err := func() ([]*jira.Transition, error) {
		s := cmdutil.Info("Fetching available transitions...")
		defer s.Stop()

		return api.ProxyTransitions(client, key)
	}()
	cmdutil.ExitIfError(err)

	if out := format(filterAvailable(all, installation), plain); out != "" {
		fmt.Println(out)
	}
}

// Jira API v2 doesn't return "isAvailable", so only Cloud transitions can be filtered,
// the same way "issue move" treats them.
func filterAvailable(all []*jira.Transition, installation string) []*jira.Transition {
	available := make([]*jira.Transition, 0, len(all))
	for _, t := range all {
		if installation == jira.InstallationTypeCloud && !t.IsAvailable {
			continue
		}
		available = append(available, t)
	}
	return available
}

func format(transitions []*jira.Transition, plain bool) string {
	sep := " -> "
	if plain {
		sep = "\t"
	}

	lines := make([]string, 0, len(transitions))
	for _, t := range transitions {
		// Plain output keeps the empty column so scripts always get two fields.
		if t.To.Name == "" && !plain {
			lines = append(lines, t.Name)
			continue
		}
		lines = append(lines, t.Name+sep+t.To.Name)
	}
	return strings.Join(lines, "\n")
}
