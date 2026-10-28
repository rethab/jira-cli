package worklog

import (
	"github.com/spf13/cobra"

	"github.com/rethab/jira-cli/internal/cmd/issue/worklog/add"
	del "github.com/rethab/jira-cli/internal/cmd/issue/worklog/delete"
	"github.com/rethab/jira-cli/internal/cmd/issue/worklog/edit"
	"github.com/rethab/jira-cli/internal/cmd/issue/worklog/list"
)

const helpText = `Worklog command helps you manage issue worklogs. See available commands below.`

// NewCmdWorklog is a worklog command.
func NewCmdWorklog() *cobra.Command {
	cmd := cobra.Command{
		Use:     "worklog",
		Short:   "Manage issue worklog",
		Long:    helpText,
		Aliases: []string{"wlg"},
		RunE:    worklog,
	}

	cmd.AddCommand(
		add.NewCmdWorklogAdd(), list.NewCmdWorklogList(), edit.NewCmdWorklogEdit(), del.NewCmdWorklogDelete(),
	)

	return &cmd
}

func worklog(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
