package changelog

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `Changelog prints the history of an issue, one line per changed field, oldest first.

Each line shows when the change happened, who made it, the field, and the old and new value.`
	examples = `$ jira issue changelog ISSUE-1

# Only the status changes
$ jira issue changelog ISSUE-1 --field status

# Tab separated "created", "author", "field", "from" and "to", for scripts
$ jira issue changelog ISSUE-1 --field status --plain`

	jiraTimeLayout = "2006-01-02T15:04:05.000-0700"
	displayLayout  = "2006-01-02 15:04"
	noAuthor       = "Automation"
)

// NewCmdChangelog is a changelog command.
func NewCmdChangelog() *cobra.Command {
	cmd := cobra.Command{
		Use:     "changelog ISSUE-KEY",
		Short:   "Show the change history of an issue",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"history"},
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"help:args": `ISSUE-KEY	Issue key, eg: ISSUE-1`,
		},
		Run: changelog,
	}

	cmd.Flags().SortFlags = false

	cmd.Flags().String("field", "", "Only show changes of this field, eg: status")
	cmd.Flags().Bool("plain", false, "Print tab separated created, author, field, from and to")

	return &cmd
}

func changelog(cmd *cobra.Command, args []string) {
	project := viper.GetString("project.key")
	key := cmdutil.GetJiraIssueKey(project, args[0])

	field, err := cmd.Flags().GetString("field")
	cmdutil.ExitIfError(err)

	plain, err := cmd.Flags().GetBool("plain")
	cmdutil.ExitIfError(err)

	client := api.DefaultClient(viper.GetBool("debug"))

	histories, err := func() ([]jira.ChangelogHistory, error) {
		s := cmdutil.Info("Fetching changelog...")
		defer s.Stop()

		return api.ProxyChangelog(client, key)
	}()
	cmdutil.ExitIfError(err)

	if out := format(filter(histories, field), plain); out != "" {
		fmt.Println(out)
	}
}

// filter keeps only the items of the given field, so a history that changed several fields at once
// doesn't bring along the unrelated ones. An empty field keeps everything.
func filter(histories []jira.ChangelogHistory, field string) []jira.ChangelogHistory {
	if field == "" {
		return histories
	}

	out := make([]jira.ChangelogHistory, 0, len(histories))
	for _, h := range histories {
		var items []jira.ChangelogItem
		for _, it := range h.Items {
			if strings.EqualFold(it.Field, field) || strings.EqualFold(it.FieldID, field) {
				items = append(items, it)
			}
		}
		if len(items) > 0 {
			h.Items = items
			out = append(out, h)
		}
	}
	return out
}

func format(histories []jira.ChangelogHistory, plain bool) string {
	var lines []string

	for _, h := range histories {
		for _, it := range h.Items {
			from, to := value(it.FromString, it.From), value(it.ToString, it.To)

			if plain {
				lines = append(lines, strings.Join([]string{h.Created, author(h.Author), it.Field, from, to}, "\t"))
				continue
			}

			lines = append(lines, strings.Join([]string{
				created(h.Created), author(h.Author), it.Field, dash(from) + " -> " + dash(to),
			}, "\t"))
		}
	}

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

func created(s string) string {
	t, err := time.Parse(jiraTimeLayout, s)
	if err != nil {
		return s
	}
	return t.Local().Format(displayLayout)
}

// Jira omits the author for some system changes.
func author(u jira.User) string {
	switch {
	case u.DisplayName != "":
		return u.DisplayName
	case u.Name != "":
		return u.Name
	default:
		return noAuthor
	}
}

func value(display, id string) string {
	if display != "" {
		return display
	}
	return id
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
