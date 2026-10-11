package create

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `Create a version (release) in a project.`
	examples = `# Create a version in the configured project
$ jira release create --name "v1.0"

# Create a version with full details
$ jira release create --name "v2.0" --description "Major release" --start-date 2024-11-01 --release-date 2024-12-31

# Create a version in another project
$ jira release create -pPRJ --name "v1.5"`
)

// NewCmdCreate is a create command.
func NewCmdCreate() *cobra.Command {
	cmd := cobra.Command{
		Use:     "create",
		Short:   "Create a version in a project",
		Long:    helpText,
		Example: examples,
		Run:     create,
	}

	cmd.Flags().String("name", "", "Version name")
	cmd.Flags().String("description", "", "Version description")
	cmd.Flags().String("start-date", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().String("release-date", "", "Release date (YYYY-MM-DD)")
	cmd.Flags().Bool("released", false, "Mark version as released")
	cmd.Flags().Bool("archived", false, "Mark version as archived")

	_ = cmd.MarkFlagRequired("name")

	return &cmd
}

func create(cmd *cobra.Command, _ []string) {
	project := viper.GetString("project.key")
	debug := viper.GetBool("debug")

	if project == "" {
		cmdutil.Failed("Project is required, use the `--project` flag or run `jira init`")
	}

	flags := cmd.Flags()

	name, err := flags.GetString("name")
	cmdutil.ExitIfError(err)

	description, err := flags.GetString("description")
	cmdutil.ExitIfError(err)

	startDate, err := flags.GetString("start-date")
	cmdutil.ExitIfError(err)

	releaseDate, err := flags.GetString("release-date")
	cmdutil.ExitIfError(err)

	released, err := flags.GetBool("released")
	cmdutil.ExitIfError(err)

	archived, err := flags.GetBool("archived")
	cmdutil.ExitIfError(err)

	version, err := func() (*jira.ProjectVersion, error) {
		s := cmdutil.Info("Creating version...")
		defer s.Stop()

		return api.DefaultClient(debug).CreateVersion(&jira.VersionCreateRequest{
			Name:        name,
			Description: description,
			Project:     project,
			StartDate:   startDate,
			ReleaseDate: releaseDate,
			Released:    released,
			Archived:    archived,
		})
	}()
	cmdutil.ExitIfError(err)

	cmdutil.Success("Version created: %s (ID: %s)", version.Name, version.ID)
}
