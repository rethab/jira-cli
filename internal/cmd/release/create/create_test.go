package create

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func TestNewRequest(t *testing.T) {
	cases := []struct {
		name    string
		project string
		flags   map[string]string
		want    *jira.VersionCreateRequest
		wantErr string
	}{
		{
			name:    "only name sets project key",
			project: "PRJ",
			flags:   map[string]string{"name": "v1.0"},
			want:    &jira.VersionCreateRequest{Name: "v1.0", Project: "PRJ"},
		},
		{
			name:    "numeric project goes to projectId",
			project: "1000",
			flags:   map[string]string{"name": "v1.0"},
			want:    &jira.VersionCreateRequest{Name: "v1.0", ProjectID: 1000},
		},
		{
			name:    "all fields",
			project: "PRJ",
			flags: map[string]string{
				"name":         " v2.0 ",
				"description":  "Major release",
				"start-date":   "2024-11-01",
				"release-date": "2024-12-31",
				"released":     "true",
				"archived":     "true",
			},
			want: &jira.VersionCreateRequest{
				Name:        "v2.0",
				Description: "Major release",
				Project:     "PRJ",
				StartDate:   "2024-11-01",
				ReleaseDate: "2024-12-31",
				Released:    true,
				Archived:    true,
			},
		},
		{
			name:    "blank name",
			project: "PRJ",
			flags:   map[string]string{"name": "  "},
			wantErr: "version name must not be empty",
		},
		{
			name:    "invalid start date",
			project: "PRJ",
			flags:   map[string]string{"name": "v1.0", "start-date": "01/11/2024"},
			wantErr: `invalid --start-date "01/11/2024": expected format YYYY-MM-DD`,
		},
		{
			name:    "invalid release date",
			project: "PRJ",
			flags:   map[string]string{"name": "v1.0", "release-date": "2024-13-01"},
			wantErr: `invalid --release-date "2024-13-01": expected format YYYY-MM-DD`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmdCreate()
			for k, v := range tc.flags {
				assert.NoError(t, cmd.Flags().Set(k, v))
			}

			got, err := newRequest(cmd, tc.project)
			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
