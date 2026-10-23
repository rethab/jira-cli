package attachment

import (
	"github.com/spf13/cobra"

	"github.com/rethab/jira-cli/internal/cmd/issue/attachment/add"
	"github.com/rethab/jira-cli/internal/cmd/issue/attachment/download"
	"github.com/rethab/jira-cli/internal/cmd/issue/attachment/list"
)

const helpText = `Attachment command helps you manage issue attachments. See available commands below.`

// NewCmdAttachment is an attachment command.
func NewCmdAttachment() *cobra.Command {
	cmd := cobra.Command{
		Use:     "attachment",
		Short:   "Manage issue attachments",
		Long:    helpText,
		Aliases: []string{"attachments"},
		RunE:    attachment,
	}

	cmd.AddCommand(add.NewCmdAttachmentAdd(), list.NewCmdAttachmentList(), download.NewCmdAttachmentDownload())

	return &cmd
}

func attachment(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
