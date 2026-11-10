package list

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func TestFormat(t *testing.T) {
	links := []*jira.RemoteLink{
		{ID: 10000, Object: jira.RemoteLinkObject{URL: "https://example.com", Title: "Example"}},
		{ID: 10001, Object: jira.RemoteLinkObject{
			URL:   "https://example.atlassian.net/wiki/x",
			Title: "Architecture\tnotes\nv2",
		}},
	}

	t.Run("plain", func(t *testing.T) {
		assert.Equal(
			t,
			"10000\tExample\thttps://example.com\n10001\tArchitecture notes v2\thttps://example.atlassian.net/wiki/x",
			format(links, true),
		)
	})

	t.Run("pretty", func(t *testing.T) {
		assert.Equal(
			t,
			"ID     TITLE                  URL\n"+
				"10000  Example                https://example.com\n"+
				"10001  Architecture notes v2  https://example.atlassian.net/wiki/x",
			format(links, false),
		)
	})

	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, format(nil, false))
		assert.Empty(t, format(nil, true))
	})
}
