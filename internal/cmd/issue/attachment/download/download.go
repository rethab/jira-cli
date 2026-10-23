package download

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	helpText = `Download downloads the attachments of an issue.

Without NAME or ID, all attachments of the issue are downloaded. Existing files are never
overwritten unless '--overwrite' is passed; the command fails before downloading anything
if a target file already exists.

If an issue has several attachments with the same name, their ids are added to the file names.`
	examples = `# Download all attachments to the current directory
$ jira issue attachment download ISSUE-1

# Download specific attachments by file name or id
$ jira issue attachment download ISSUE-1 report.pdf 10042

# Download to a different directory, which is created if needed
$ jira issue attachment download ISSUE-1 -o /path/to/dir

# Replace files that already exist
$ jira issue attachment download ISSUE-1 --overwrite`

	dirPerm = 0o755
)

// NewCmdAttachmentDownload is an attachment download command.
func NewCmdAttachmentDownload() *cobra.Command {
	cmd := cobra.Command{
		Use:     "download ISSUE-KEY [NAME|ID...]",
		Short:   "Download attachments of an issue",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"get"},
		Args:    cobra.MinimumNArgs(1),
		Annotations: map[string]string{
			"help:args": "ISSUE-KEY\tIssue key of the source issue, eg: ISSUE-1\n" +
				"NAME|ID\tFile name or id of an attachment, see 'jira issue attachment list'",
		},
		Run: download,
	}

	cmd.Flags().SortFlags = false

	cmd.Flags().StringP("output", "o", ".", "Directory to download the attachments to")
	cmd.Flags().Bool("overwrite", false, "Overwrite files that already exist")

	return &cmd
}

func download(cmd *cobra.Command, args []string) {
	issueKey := cmdutil.GetJiraIssueKey(viper.GetString("project.key"), args[0])

	dir, err := cmd.Flags().GetString("output")
	cmdutil.ExitIfError(err)

	overwrite, err := cmd.Flags().GetBool("overwrite")
	cmdutil.ExitIfError(err)

	client := api.DefaultClient(viper.GetBool("debug"))

	all, err := func() ([]jira.Attachment, error) {
		s := cmdutil.Info("Fetching attachments")
		defer s.Stop()

		return client.GetIssueAttachments(issueKey)
	}()
	cmdutil.ExitIfError(err)

	if len(all) == 0 {
		cmdutil.Failed("No attachments found for issue %q", issueKey)
	}

	selected, err := selectAttachments(all, args[1:])
	cmdutil.ExitIfError(err)

	names := targetNames(selected)

	cmdutil.ExitIfError(os.MkdirAll(dir, dirPerm))

	if !overwrite {
		var existing []string
		for _, n := range names {
			if _, err := os.Lstat(filepath.Join(dir, n)); err == nil {
				existing = append(existing, n)
			}
		}
		if len(existing) > 0 {
			cmdutil.Failed("Not overwriting existing files in %s: %s\nUse --overwrite to replace them", dir, strings.Join(existing, ", "))
		}
	}

	for i, a := range selected {
		target := filepath.Join(dir, names[i])

		err := func() error {
			s := cmdutil.Info(fmt.Sprintf("Downloading %s", a.Filename))
			defer s.Stop()

			return save(client, a, target, overwrite)
		}()
		cmdutil.ExitIfError(err)

		cmdutil.Success("Downloaded %q to %s", a.Filename, target)
	}
}

// save streams the attachment to target. Without overwrite the file is
// created exclusively, so a file that appeared since the existence check
// still isn't clobbered.
func save(client *jira.Client, a jira.Attachment, target string, overwrite bool) (err error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	const perm = 0o644

	f, err := os.OpenFile(target, flags, perm)
	if err != nil {
		return err
	}

	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(target)
		}
	}()

	return client.DownloadAttachment(a, f)
}

// selectAttachments returns the attachments matching refs, each of which is
// an attachment id or file name. A name can match several attachments, as
// Jira doesn't enforce unique names. Without refs, all attachments are returned.
func selectAttachments(all []jira.Attachment, refs []string) ([]jira.Attachment, error) {
	if len(refs) == 0 {
		return all, nil
	}

	var (
		selected []jira.Attachment
		seen     = make(map[string]bool)
	)

	for _, ref := range refs {
		var matched bool

		for _, a := range all {
			if a.ID != ref && a.Filename != ref {
				continue
			}
			matched = true

			if !seen[a.ID] {
				seen[a.ID] = true
				selected = append(selected, a)
			}
		}

		if !matched {
			return nil, fmt.Errorf("no attachment with name or id %q", ref)
		}
	}
	return selected, nil
}

// targetNames returns the local file name for each attachment. Names that
// would collide get the attachment id added, so that downloading all
// attachments of an issue never makes one overwrite another.
func targetNames(attachments []jira.Attachment) []string {
	base := make([]string, len(attachments))
	count := make(map[string]int)

	for i, a := range attachments {
		base[i] = safeName(a)
		count[base[i]]++
	}

	names := make([]string, len(attachments))
	for i, a := range attachments {
		names[i] = base[i]
		if count[base[i]] > 1 {
			ext := filepath.Ext(base[i])
			names[i] = strings.TrimSuffix(base[i], ext) + "-" + a.ID + ext
		}
	}
	return names
}

// safeName reduces the file name to a single path element, since it comes
// from the server and must not be able to point outside the target directory.
func safeName(a jira.Attachment) string {
	name := filepath.Base(strings.ReplaceAll(a.Filename, `\`, "/"))
	if name == "." || name == ".." || name == string(filepath.Separator) {
		return "attachment-" + a.ID
	}
	return name
}
