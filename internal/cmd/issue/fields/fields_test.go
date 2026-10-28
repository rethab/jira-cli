package fields

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func newField(id, name, dataType, items string, required bool, values ...jira.CreateMetaAllowedValue) *jira.CreateMetaField {
	f := &jira.CreateMetaField{FieldID: id, Name: name, Required: required, AllowedValues: values}
	f.Schema.DataType = dataType
	f.Schema.Items = items
	return f
}

func testFields() []*jira.CreateMetaField {
	return []*jira.CreateMetaField{
		newField("summary", "Summary", "string", "", true),
		newField("priority", "Priority", "priority", "", false,
			jira.CreateMetaAllowedValue{Name: "High"}, jira.CreateMetaAllowedValue{Name: "Low"}),
		newField("customfield_14695", "Environments Affected", "array", "option", true,
			jira.CreateMetaAllowedValue{Value: "Development"}, jira.CreateMetaAllowedValue{Value: "Production"}),
		newField("customfield_14694", "Platform Component", "option-with-child", "", true,
			jira.CreateMetaAllowedValue{
				Value: "Pipelines/Scripts",
				Children: []jira.CreateMetaAllowedValue{
					{Value: "Environment Management"}, {Value: "Release"},
				},
			},
			jira.CreateMetaAllowedValue{Value: "Standalone"},
		),
	}
}

func TestFormatHuman(t *testing.T) {
	expected := `Summary (summary): string, required

Priority (priority): priority
  High
  Low

Environments Affected (customfield_14695): array<option>, required
  Development
  Production

Platform Component (customfield_14694): option-with-child, required
  Pipelines/Scripts->Environment Management
  Pipelines/Scripts->Release
  Standalone`

	assert.Equal(t, expected, format(testFields(), false))
}

func TestFormatPlain(t *testing.T) {
	expected := "summary\tSummary\tstring\ttrue\t\n" +
		"priority\tPriority\tpriority\tfalse\tHigh\n" +
		"priority\tPriority\tpriority\tfalse\tLow\n" +
		"customfield_14695\tEnvironments Affected\tarray<option>\ttrue\tDevelopment\n" +
		"customfield_14695\tEnvironments Affected\tarray<option>\ttrue\tProduction\n" +
		"customfield_14694\tPlatform Component\toption-with-child\ttrue\tPipelines/Scripts->Environment Management\n" +
		"customfield_14694\tPlatform Component\toption-with-child\ttrue\tPipelines/Scripts->Release\n" +
		"customfield_14694\tPlatform Component\toption-with-child\ttrue\tStandalone"

	assert.Equal(t, expected, format(testFields(), true))
}

func TestFormatEmpty(t *testing.T) {
	assert.Empty(t, format(nil, false))
	assert.Empty(t, format(nil, true))
}

func TestFormatFallsBackToKeyWhenThereIsNoFieldID(t *testing.T) {
	f := newField("", "Reason", "option", "", false)
	f.Key = "customfield_12345"

	assert.Equal(t, "Reason (customfield_12345): option", format([]*jira.CreateMetaField{f}, false))
}

func TestFilterRequired(t *testing.T) {
	got := filterRequired(testFields())

	ids := make([]string, 0, len(got))
	for _, f := range got {
		ids = append(ids, f.ID())
	}
	assert.Equal(t, []string{"summary", "customfield_14695", "customfield_14694"}, ids)
}

func TestUseLegacyCreateMeta(t *testing.T) {
	cases := []struct {
		name         string
		installation string
		major, minor int
		legacy       bool
	}{
		{name: "cloud never uses the deprecated endpoint", installation: jira.InstallationTypeCloud, legacy: false},
		{name: "server 9", installation: jira.InstallationTypeLocal, major: 9, minor: 0, legacy: false},
		{name: "server 8.5", installation: jira.InstallationTypeLocal, major: 8, minor: 5, legacy: false},
		{name: "server 8.4", installation: jira.InstallationTypeLocal, major: 8, minor: 4, legacy: true},
		{name: "server 7", installation: jira.InstallationTypeLocal, major: 7, minor: 13, legacy: true},
		{name: "server with unknown version", installation: jira.InstallationTypeLocal, legacy: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.legacy, useLegacyCreateMeta(tc.installation, tc.major, tc.minor))
		})
	}
}
