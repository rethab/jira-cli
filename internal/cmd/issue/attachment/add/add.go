package add

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `Add uploads one or more files to an issue and prints the id and name of each new attachment.`
	examples = `$ jira issue attachment add ISSUE-1 screenshot.png

# Upload several files at once
$ jira issue attachment add ISSUE-1 report.pdf logs/*.txt`
)

// NewCmdAttachmentAdd is an attachment add command.
func NewCmdAttachmentAdd() *cobra.Command {
	cmd := cobra.Command{
		Use:     "add ISSUE-KEY FILE [FILE...]",
		Short:   "Add attachments to an issue",
		Long:    helpText,
		Example: examples,
		Args:    cobra.MinimumNArgs(2),
		Annotations: map[string]string{
			"help:args": "ISSUE-KEY\tIssue key of the source issue, eg: ISSUE-1\n" +
				"FILE\tPath of a file to upload",
		},
		Run: add,
	}

	return &cmd
}

func add(_ *cobra.Command, args []string) {
	issueKey := cmdutil.GetJiraIssueKey(viper.GetString("project.key"), args[0])
	files := args[1:]

	// Checking every file up front keeps a typo in the last argument from
	// leaving the earlier files uploaded.
	for _, f := range files {
		cmdutil.ExitIfError(checkFile(f))
	}

	client := api.DefaultClient(viper.GetBool("debug"))

	for _, f := range files {
		attached, err := func() ([]jira.Attachment, error) {
			s := cmdutil.Info(fmt.Sprintf("Uploading %s", f))
			defer s.Stop()

			return upload(client, issueKey, f)
		}()
		cmdutil.ExitIfError(err)

		for _, a := range attached {
			cmdutil.Success("Attached %q to issue %q with id %s", a.Filename, issueKey, a.ID)
		}
	}
}

func checkFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}

func upload(client *jira.Client, issueKey, path string) ([]jira.Attachment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return client.AddIssueAttachment(issueKey, path, f)
}
