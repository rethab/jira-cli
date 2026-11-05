package transitions

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `Transitions lists the transitions an issue can take from its current state, without moving it.

Each line shows the transition name and the status it leads to, as in "Start work -> In Progress".
With --fields, transitions that open a screen are marked, and the fields that screen asks for are listed below them.`
	examples = `$ jira issue transitions ISSUE-1

# Tab separated "transition name" and "target status", for scripts
$ jira issue transitions ISSUE-1 --plain

# Also list the fields each transition asks for, eg. a required resolution
$ jira issue transitions ISSUE-1 --fields

# Print the raw Jira API response, including transitions that are not available
$ jira issue transitions ISSUE-1 --raw

# Raw response including the fields of each transition's screen
$ jira issue transitions ISSUE-1 --raw --fields`
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
	cmd.Flags().Bool("fields", false, "Also list the fields each transition's screen asks for")
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

	withFields, err := cmd.Flags().GetBool("fields")
	cmdutil.ExitIfError(err)

	client := api.DefaultClient(viper.GetBool("debug"))

	if raw {
		// Verbatim like "issue view --raw", so unavailable transitions stay in, marked by "isAvailable".
		resp, err := func() (string, error) {
			s := cmdutil.Info("Fetching available transitions...")
			defer s.Stop()

			if withFields {
				return api.ProxyTransitionsWithFieldsRaw(client, key)
			}
			return api.ProxyTransitionsRaw(client, key)
		}()
		cmdutil.ExitIfError(err)

		fmt.Println(resp)
		return
	}

	all, err := func() ([]*jira.Transition, error) {
		s := cmdutil.Info("Fetching available transitions...")
		defer s.Stop()

		if withFields {
			return api.ProxyTransitionsWithFields(client, key)
		}
		return api.ProxyTransitions(client, key)
	}()
	cmdutil.ExitIfError(err)

	available := jira.AvailableTransitions(all, installation)

	var out string
	if withFields {
		out = formatWithFields(available, plain)
	} else {
		out = format(available, plain)
	}
	if out != "" {
		fmt.Println(out)
	}
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

// formatWithFields lists the fields of each transition's screen below it. Fields are sorted by id so the
// output is stable, as Jira returns them in no particular order.
func formatWithFields(transitions []*jira.Transition, plain bool) string {
	if plain {
		return formatFieldsPlain(transitions)
	}

	blocks := make([]string, 0, len(transitions))
	for _, t := range transitions {
		head := t.Name
		if t.To.Name != "" {
			head += " -> " + t.To.Name
		}
		if t.HasScreen {
			head += " (screen)"
		}

		var b strings.Builder
		w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		for _, id := range sortedFieldIDs(t) {
			f := t.Fields[id]
			row := []string{"    " + id, f.Name, requiredLabel(f.Required)}
			if allowed := allowedValues(f); len(allowed) > 0 {
				row = append(row, strings.Join(allowed, ", "))
			}
			_, _ = fmt.Fprintln(w, strings.Join(row, "\t"))
		}
		_ = w.Flush()

		blocks = append(blocks, strings.TrimSuffix(head+"\n"+b.String(), "\n"))
	}
	return strings.Join(blocks, "\n")
}

func formatFieldsPlain(transitions []*jira.Transition) string {
	var lines []string
	for _, t := range transitions {
		ids := sortedFieldIDs(t)
		if len(ids) == 0 {
			lines = append(lines, strings.Join([]string{t.Name, t.To.Name, "", "", "", ""}, "\t"))
			continue
		}
		for _, id := range ids {
			f := t.Fields[id]
			lines = append(lines, strings.Join([]string{
				t.Name, t.To.Name, id, f.Name, requiredLabel(f.Required), strings.Join(allowedValues(f), ","),
			}, "\t"))
		}
	}
	return strings.Join(lines, "\n")
}

func sortedFieldIDs(t *jira.Transition) []string {
	ids := make([]string, 0, len(t.Fields))
	for id := range t.Fields {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func requiredLabel(required bool) string {
	if required {
		return "required"
	}
	return "optional"
}

// System fields like resolution identify their options by name, custom option fields by value.
func allowedValues(f jira.TransitionField) []string {
	out := make([]string, 0, len(f.AllowedValues))
	for _, v := range f.AllowedValues {
		if v.Name != "" {
			out = append(out, v.Name)
		} else if v.Value != "" {
			out = append(out, v.Value)
		}
	}
	return out
}
