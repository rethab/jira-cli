package changelog

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aymanbagabas/go-udiff"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `Changelog prints the history of an issue, one line per changed field, oldest first.

Each line shows when the change happened, who made it, the field, and the old and new value.
Line breaks in a value are flattened to spaces. With --diff, each change is printed as a header
and a unified diff of the old and new value instead, which keeps the line breaks.`
	examples = `$ jira issue changelog ISSUE-1

# Only the status changes
$ jira issue changelog ISSUE-1 --field status

# Tab separated "created", "author", "field", "from" and "to", for scripts
$ jira issue changelog ISSUE-1 --field status --plain

# What changed in the description, as a unified diff per change
$ jira issue changelog ISSUE-1 --field description --diff`

	jiraTimeLayout = "2006-01-02T15:04:05.000-0700"
	displayLayout  = "2006-01-02 15:04"
	noAuthor       = "Automation"
)

// Values like a changed description span several lines, which would break one line per change.
// Only the one-line modes use it, --diff keeps the line breaks.
var flatten = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ")

// Server/DC may store \r\n, which would make every line of a diff look changed.
var normaliseLineEndings = strings.NewReplacer("\r\n", "\n", "\r", "\n")

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
	cmd.Flags().Bool("diff", false, "Print each change as a unified diff of the old and new value, keeping line breaks")

	return &cmd
}

func changelog(cmd *cobra.Command, args []string) {
	project := viper.GetString("project.key")
	key := cmdutil.GetJiraIssueKey(project, args[0])

	field, err := cmd.Flags().GetString("field")
	cmdutil.ExitIfError(err)

	plain, err := cmd.Flags().GetBool("plain")
	cmdutil.ExitIfError(err)

	diff, err := cmd.Flags().GetBool("diff")
	cmdutil.ExitIfError(err)

	if diff && plain {
		cmdutil.Failed("--diff can't be combined with --plain")
	}

	client := api.DefaultClient(viper.GetBool("debug"))

	histories, err := func() ([]jira.ChangelogHistory, error) {
		s := cmdutil.Info("Fetching changelog...")
		defer s.Stop()

		return api.ProxyChangelog(client, key)
	}()
	cmdutil.ExitIfError(err)

	histories = filter(histories, field)

	var out string
	if diff {
		out = formatDiff(histories)
	} else {
		out = format(histories, plain)
	}

	if out != "" {
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
			from, to := flatten.Replace(value(it.FromString, it.From)), flatten.Replace(value(it.ToString, it.To))

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

func formatDiff(histories []jira.ChangelogHistory) string {
	var blocks []string

	for _, h := range histories {
		for _, it := range h.Items {
			header := strings.Join([]string{created(h.Created), author(h.Author), it.Field}, "  ")

			rawFrom, rawTo := value(it.FromString, it.From), value(it.ToString, it.To)
			body := strings.TrimSuffix(udiff.Unified("from", "to", diffText(rawFrom), diffText(rawTo)), "\n")
			switch {
			case body != "":
			case rawFrom == rawTo:
				// Jira sometimes logs a change whose old and new text are the same.
				body = "(values are identical)"
			default:
				// Normalising hid the difference, so don't claim the values are the same.
				body = "(only line endings or a trailing line break differ)"
			}

			blocks = append(blocks, header+"\n"+body)
		}
	}

	return strings.Join(blocks, "\n\n")
}

// diffText also ends a non-empty value with a newline, as Jira values have none and udiff would
// otherwise add a "No newline at end of file" marker.
func diffText(s string) string {
	s = normaliseLineEndings.Replace(s)
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
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
