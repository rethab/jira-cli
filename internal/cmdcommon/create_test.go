package cmdcommon

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rethab/jira-cli/pkg/jira"
)

// jira.IssueTypeField declares only json tags, so viper cannot match the yaml keys
// `jira init` writes ("schema.datatype") against the struct's json names
// ("schema.type"). The decode survives purely on mapstructure's case-insensitive
// fallback to Go field names, which no type or tag pins down. Viper has already
// swapped its mapstructure implementation once; this test is what would catch the
// next such swap silently dropping custom fields.
func TestGetConfiguredCustomFields(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.SetConfigType("yaml")

	err := viper.ReadConfig(strings.NewReader(`
issue:
  fields:
    custom:
      - name: Story Points
        key: customfield_10016
        schema:
          datatype: number
      - name: Sprint
        key: customfield_10020
        schema:
          datatype: array
          items: string
`))
	assert.NoError(t, err)

	fields, err := GetConfiguredCustomFields()
	assert.NoError(t, err)
	assert.Len(t, fields, 2)

	assert.Equal(t, "Story Points", fields[0].Name)
	assert.Equal(t, "customfield_10016", fields[0].Key)
	assert.Equal(t, "number", fields[0].Schema.DataType)
	assert.Empty(t, fields[0].Schema.Items)

	assert.Equal(t, "Sprint", fields[1].Name)
	assert.Equal(t, "customfield_10020", fields[1].Key)
	assert.Equal(t, "array", fields[1].Schema.DataType)
	assert.Equal(t, "string", fields[1].Schema.Items)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = orig })

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func TestValidateCustomFields(t *testing.T) {
	reason := jira.IssueTypeField{Name: "Reason", Key: "customfield_12345"}
	reason.Schema.DataType = "option"
	points := jira.IssueTypeField{Name: "Story Points", Key: "customfield_10016"}
	points.Schema.DataType = "number"
	configured := []jira.IssueTypeField{reason, points}

	cases := []struct {
		name    string
		fields  map[string]string
		invalid string
	}{
		{name: "exact identifier", fields: map[string]string{"story-points": "3"}},
		{name: "name as typed in the config", fields: map[string]string{"Reason": "x"}},
		{name: "mixed case multi-word", fields: map[string]string{"Story-Points": "3"}},
		{name: "unknown field", fields: map[string]string{"Nope": "x"}, invalid: "Nope"},
		{name: "raw key is not an identifier", fields: map[string]string{"customfield_12345": "x"}, invalid: "customfield_12345"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStderr(t, func() { ValidateCustomFields(tc.fields, configured) })

			if tc.invalid == "" {
				assert.Empty(t, out)
				return
			}
			assert.Contains(t, out, "Invalid custom fields used in the command: "+tc.invalid)
		})
	}
}

func TestGetMetadataOptions(t *testing.T) {
	options := func(extra ...string) []string {
		return GetMetadata(extra...)[0].Prompt.(*survey.MultiSelect).Options
	}

	assert.NotContains(t, options(), "OriginalEstimate", "create has no handling for it")
	assert.Contains(t, options("OriginalEstimate"), "OriginalEstimate")
}
