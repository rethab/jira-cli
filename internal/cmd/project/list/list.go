package list

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/internal/view"
	"github.com/rethab/jira-cli/pkg/jira"
)

// NewCmdList is a list command.
func NewCmdList() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List lists Jira projects",
		Long:    "List lists Jira projects that a user has access to.",
		Aliases: []string{"lists", "ls"},
		Run:     List,
	}
	cmd.Flags().Bool("raw", false, "Print raw JSON output")

	return cmd
}

// List displays a list view.
func List(cmd *cobra.Command, _ []string) {
	debug, err := cmd.Flags().GetBool("debug")
	cmdutil.ExitIfError(err)

	projects, total, err := func() ([]*jira.Project, int, error) {
		s := cmdutil.Info("Fetching projects...")
		defer s.Stop()

		projects, err := api.DefaultClient(debug).Project()
		if err != nil {
			return nil, 0, err
		}
		return projects, len(projects), nil
	}()
	cmdutil.ExitIfError(err)

	if total == 0 {
		cmdutil.Failed("No projects found.")
		return
	}

	raw, err := cmd.Flags().GetBool("raw")
	cmdutil.ExitIfError(err)

	if raw {
		outputRawJSON(projects)
		return
	}

	v := view.NewProject(projects)

	cmdutil.ExitIfError(v.Render())
}

func outputRawJSON(projects []*jira.Project) {
	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		cmdutil.Failed("Failed to marshal projects to JSON: %s", err)
		return
	}
	fmt.Println(string(data))
}
