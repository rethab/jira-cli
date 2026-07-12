package edit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ankitpokhrel/jira-cli/pkg/adf"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
)

func TestBuildEditRequestConvertsMarkdownBody(t *testing.T) {
	tests := []struct {
		name        string
		description interface{} // string on-prem/Server, *adf.ADF on Cloud
		body        string
		wantBody    string
	}{
		{
			name:        "on-prem/server issue with plain string description",
			description: "Old plain description",
			body:        "# Heading\n\nSome **bold** text",
			wantBody:    "h1. Heading\nSome *bold* text\n\n",
		},
		{
			name:        "cloud issue with ADF description",
			description: &adf.ADF{Version: 1, DocType: "doc"},
			body:        "# Heading\n\nSome **bold** text",
			wantBody:    "h1. Heading\nSome *bold* text\n\n",
		},
		{
			name:        "empty body is left untouched",
			description: "Old plain description",
			body:        "",
			wantBody:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issue := &jira.Issue{
				Fields: jira.IssueFields{
					Description: tc.description,
				},
			}
			params := &editParams{body: tc.body}

			edr := buildEditRequest("TEST", issue, params)

			assert.Equal(t, tc.wantBody, edr.Body)
		})
	}
}
