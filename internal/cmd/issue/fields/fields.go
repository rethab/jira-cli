package fields

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
	helpText = `Fields lists the fields on the create screen of an issue type, to find out what 'jira issue create' needs.

Each field is shown with its id, its type, whether it is required, and the values it accepts. The values are written
the way '--custom' takes them, so a cascading select shows one "Parent->Child" line per child. The id can be used as
the '--custom' key, which also works for fields that 'jira init' did not pick up.`
	examples = `$ jira issue fields -t Problem

# Only the fields a create can't do without
$ jira issue fields -p PROJ -t Problem --required

# Tab separated ID, NAME, TYPE, REQUIRED and VALUE, one row per allowed value, for scripts
$ jira issue fields -t Problem --plain`
)

// NewCmdFields is a fields command.
func NewCmdFields() *cobra.Command {
	cmd := cobra.Command{
		Use:     "fields",
		Short:   "List the fields of an issue type's create screen",
		Long:    helpText,
		Example: examples,
		Args:    cobra.NoArgs,
		Run:     fields,
	}

	cmd.Flags().SortFlags = false

	cmd.Flags().StringP("type", "t", "", "Issue type")
	cmd.Flags().Bool("required", false, "Only list the required fields")
	cmd.Flags().Bool("plain", false, "Print tab separated ID, NAME, TYPE, REQUIRED and VALUE, one row per allowed value")

	_ = cmd.MarkFlagRequired("type")

	return &cmd
}

func fields(cmd *cobra.Command, _ []string) {
	project := viper.GetString("project.key")

	issueType, err := cmd.Flags().GetString("type")
	cmdutil.ExitIfError(err)

	required, err := cmd.Flags().GetBool("required")
	cmdutil.ExitIfError(err)

	plain, err := cmd.Flags().GetBool("plain")
	cmdutil.ExitIfError(err)

	client := api.DefaultClient(viper.GetBool("debug"))
	legacy := useLegacyCreateMeta(
		viper.GetString("installation"), viper.GetInt("version.major"), viper.GetInt("version.minor"),
	)

	all, err := func() ([]*jira.CreateMetaField, error) {
		s := cmdutil.Info("Fetching fields...")
		defer s.Stop()

		return client.GetCreateFields(project, issueType, legacy)
	}()
	cmdutil.ExitIfError(err)

	if required {
		all = filterRequired(all)
	}

	if out := format(all, plain); out != "" {
		fmt.Println(out)
	}
}

// Same cut-off as the config generator: the per-issue-type createmeta endpoints exist from
// Jira Server 8.5. Cloud always has them, and its legacy endpoint is deprecated.
func useLegacyCreateMeta(installation string, major, minor int) bool {
	if installation != jira.InstallationTypeLocal {
		return false
	}
	return major < 8 || (major == 8 && minor < 5)
}

func filterRequired(all []*jira.CreateMetaField) []*jira.CreateMetaField {
	out := make([]*jira.CreateMetaField, 0, len(all))
	for _, f := range all {
		if f.Required {
			out = append(out, f)
		}
	}
	return out
}

func format(fields []*jira.CreateMetaField, plain bool) string {
	blocks := make([]string, 0, len(fields))

	for _, f := range fields {
		values := allowedValues(f.AllowedValues)

		if plain {
			blocks = append(blocks, formatPlain(f, values))
			continue
		}
		blocks = append(blocks, formatHuman(f, values))
	}

	sep := "\n\n"
	if plain {
		sep = "\n"
	}
	return strings.Join(blocks, sep)
}

func formatHuman(f *jira.CreateMetaField, values []string) string {
	header := fmt.Sprintf("%s (%s): %s", f.Name, f.ID(), fieldType(f))
	if f.Required {
		header += ", required"
	}

	lines := make([]string, 0, len(values)+1)
	lines = append(lines, header)
	for _, v := range values {
		lines = append(lines, "  "+v)
	}
	return strings.Join(lines, "\n")
}

// Fields without allowed values still get one row, so a script sees every field.
func formatPlain(f *jira.CreateMetaField, values []string) string {
	if len(values) == 0 {
		values = []string{""}
	}

	rows := make([]string, 0, len(values))
	for _, v := range values {
		rows = append(rows, strings.Join([]string{f.ID(), f.Name, fieldType(f), fmt.Sprint(f.Required), v}, "\t"))
	}
	return strings.Join(rows, "\n")
}

func fieldType(f *jira.CreateMetaField) string {
	if f.Schema.DataType == "array" && f.Schema.Items != "" {
		return fmt.Sprintf("array<%s>", f.Schema.Items)
	}
	return f.Schema.DataType
}

// allowedValues writes the values the way '--custom' accepts them: "value" for options,
// "name" for system fields, and "Parent->Child" for each child of a cascading select.
func allowedValues(values []jira.CreateMetaAllowedValue) []string {
	out := make([]string, 0, len(values))

	for _, v := range values {
		label := v.Value
		if label == "" {
			label = v.Name
		}

		if len(v.Children) == 0 {
			out = append(out, label)
			continue
		}
		for _, c := range v.Children {
			child := c.Value
			if child == "" {
				child = c.Name
			}
			out = append(out, label+"->"+child)
		}
	}

	return out
}
