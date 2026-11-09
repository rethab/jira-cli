package types

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `Types lists the issue link types of your Jira instance, with their outward and inward wording.

NAME is the value "jira issue link" takes as its link type. "jira issue link A B NAME" reads "A <outward> B",
and B reads "B <inward> A".`
	examples = `$ jira issue link types

# Tab separated name, outward and inward wording, for scripts
$ jira issue link types --plain`
)

// NewCmdTypes is a link types command.
func NewCmdTypes() *cobra.Command {
	cmd := cobra.Command{
		Use:     "types",
		Short:   "List the issue link types",
		Long:    helpText,
		Example: examples,
		Args:    cobra.NoArgs,
		Run:     types,
	}

	cmd.Flags().Bool("plain", false, "Print tab separated name, outward and inward wording")

	return &cmd
}

func types(cmd *cobra.Command, _ []string) {
	plain, err := cmd.Flags().GetBool("plain")
	cmdutil.ExitIfError(err)

	client := api.DefaultClient(viper.GetBool("debug"))

	lt, err := func() ([]*jira.IssueLinkType, error) {
		s := cmdutil.Info("Fetching link types...")
		defer s.Stop()

		return client.GetIssueLinkTypes()
	}()
	cmdutil.ExitIfError(err)

	if out := format(lt, plain); out != "" {
		fmt.Println(out)
	}
}

func format(types []*jira.IssueLinkType, plain bool) string {
	if len(types) == 0 {
		return ""
	}

	// tabwriter would pad the columns, and scripts need the bare tabs.
	if plain {
		lines := make([]string, 0, len(types))
		for _, t := range types {
			lines = append(lines, t.Name+"\t"+t.Outward+"\t"+t.Inward)
		}
		return strings.Join(lines, "\n")
	}

	var b strings.Builder

	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tOUTWARD\tINWARD")
	for _, t := range types {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", t.Name, t.Outward, t.Inward)
	}
	_ = w.Flush()

	return strings.TrimSuffix(b.String(), "\n")
}
