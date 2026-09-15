package worklog

import (
	"github.com/spf13/cobra"

	"github.com/rethab/jira-cli/internal/cmd/worklog/list"
)

const helpText = `Worklog lets you inspect worklogs logged across issues. See available commands below.`

// NewCmdWorklog is a worklog command.
func NewCmdWorklog() *cobra.Command {
	cmd := cobra.Command{
		Use:         "worklog",
		Short:       "Inspect worklogs across issues",
		Long:        helpText,
		Annotations: map[string]string{"cmd:main": "true"},
		RunE:        worklog,
	}

	cmd.AddCommand(list.NewCmdWorklogList())

	return &cmd
}

func worklog(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
