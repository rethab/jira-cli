package create

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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

	req, err := newRequest(cmd, project)
	cmdutil.ExitIfError(err)

	version, err := func() (*jira.ProjectVersion, error) {
		s := cmdutil.Info("Creating version...")
		defer s.Stop()

		return api.DefaultClient(debug).CreateVersion(req)
	}()
	cmdutil.ExitIfError(err)

	cmdutil.Success("Version created: %s (ID: %s)", version.Name, version.ID)
}

func newRequest(cmd *cobra.Command, project string) (*jira.VersionCreateRequest, error) {
	flags := cmd.Flags()

	name, err := flags.GetString("name")
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("version name must not be empty")
	}

	description, err := flags.GetString("description")
	if err != nil {
		return nil, err
	}

	startDate, err := dateFlag(cmd, "start-date")
	if err != nil {
		return nil, err
	}

	releaseDate, err := dateFlag(cmd, "release-date")
	if err != nil {
		return nil, err
	}

	released, err := flags.GetBool("released")
	if err != nil {
		return nil, err
	}

	archived, err := flags.GetBool("archived")
	if err != nil {
		return nil, err
	}

	req := jira.VersionCreateRequest{
		Name:        name,
		Description: description,
		StartDate:   startDate,
		ReleaseDate: releaseDate,
		Released:    released,
		Archived:    archived,
	}
	// The "project" field only accepts a key; numeric IDs (allowed by `release list`) must go in projectId.
	if id, err := strconv.ParseInt(project, 10, 64); err == nil {
		req.ProjectID = id
	} else {
		req.Project = project
	}

	return &req, nil
}

func dateFlag(cmd *cobra.Command, flag string) (string, error) {
	value, err := cmd.Flags().GetString(flag)
	if err != nil || value == "" {
		return value, err
	}
	if _, err := time.Parse(cmdutil.DateLayout, value); err != nil {
		return "", fmt.Errorf("invalid --%s %q: expected format YYYY-MM-DD", flag, value)
	}
	return value, nil
}
