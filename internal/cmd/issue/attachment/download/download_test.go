package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rethab/jira-cli/pkg/jira"
)

func TestSave(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/attachment/content/broken" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("new"))
	}))
	defer server.Close()

	client := jira.NewClient(jira.Config{Server: server.URL}, jira.WithTimeout(3*time.Second))
	ok := jira.Attachment{Filename: "a.txt", Content: server.URL + "/rest/api/2/attachment/content/ok"}
	broken := jira.Attachment{Filename: "a.txt", Content: server.URL + "/rest/api/2/attachment/content/broken"}

	setup := func(t *testing.T) (string, string) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a.txt")
		require.NoError(t, os.WriteFile(target, []byte("old"), filePerm))
		return dir, target
	}
	assertOnly := func(t *testing.T, dir, target, content string) {
		b, err := os.ReadFile(target)
		require.NoError(t, err)
		assert.Equal(t, content, string(b))

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "no temporary file is left behind")
	}

	t.Run("existing file is kept without overwrite", func(t *testing.T) {
		dir, target := setup(t)
		assert.ErrorIs(t, save(client, ok, target, false), os.ErrExist)
		assertOnly(t, dir, target, "old")
	})

	t.Run("existing file is replaced with overwrite", func(t *testing.T) {
		dir, target := setup(t)
		assert.NoError(t, save(client, ok, target, true))
		assertOnly(t, dir, target, "new")
	})

	t.Run("failed download with overwrite keeps the existing file", func(t *testing.T) {
		dir, target := setup(t)
		assert.Error(t, save(client, broken, target, true))
		assertOnly(t, dir, target, "old")
	})

	t.Run("failed download leaves nothing behind", func(t *testing.T) {
		dir := t.TempDir()
		assert.Error(t, save(client, broken, filepath.Join(dir, "a.txt"), false))

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
}

func TestSelectAttachments(t *testing.T) {
	all := []jira.Attachment{
		{ID: "10", Filename: "a.txt"},
		{ID: "11", Filename: "b.txt"},
		{ID: "12", Filename: "a.txt"},
	}

	t.Run("all without refs", func(t *testing.T) {
		out, err := selectAttachments(all, nil)
		assert.NoError(t, err)
		assert.Equal(t, all, out)
	})

	t.Run("by id and by name", func(t *testing.T) {
		out, err := selectAttachments(all, []string{"11", "a.txt"})
		assert.NoError(t, err)
		assert.Equal(t, []jira.Attachment{all[1], all[0], all[2]}, out)
	})

	t.Run("same attachment named twice is selected once", func(t *testing.T) {
		out, err := selectAttachments(all, []string{"b.txt", "11"})
		assert.NoError(t, err)
		assert.Equal(t, []jira.Attachment{all[1]}, out)
	})

	t.Run("unknown ref", func(t *testing.T) {
		_, err := selectAttachments(all, []string{"11", "nope.txt"})
		assert.ErrorContains(t, err, `"nope.txt"`)
	})
}

func TestTargetNames(t *testing.T) {
	names := targetNames([]jira.Attachment{
		{ID: "10", Filename: "a.txt"},
		{ID: "11", Filename: "b.txt"},
		{ID: "12", Filename: "a.txt"},
		{ID: "13", Filename: "../../etc/passwd"},
		{ID: "14", Filename: ".."},
		{ID: "15", Filename: `..\evil.sh`},
	})

	assert.Equal(t, []string{"a-10.txt", "b.txt", "a-12.txt", "passwd", "attachment-14", "evil.sh"}, names)
}

func TestTargetNamesCaseInsensitive(t *testing.T) {
	names := targetNames([]jira.Attachment{
		{ID: "10", Filename: "Report.pdf"},
		{ID: "11", Filename: "report.pdf"},
	})

	assert.Equal(t, []string{"Report-10.pdf", "report-11.pdf"}, names)
}

func TestTargetNamesSuffixDoesNotClash(t *testing.T) {
	names := targetNames([]jira.Attachment{
		{ID: "10", Filename: "a.txt"},
		{ID: "12", Filename: "a.txt"},
		{ID: "99", Filename: "a-10.txt"},
	})

	assert.Equal(t, []string{"a-10-2.txt", "a-12.txt", "a-10.txt"}, names)
}
