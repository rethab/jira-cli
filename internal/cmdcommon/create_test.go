package cmdcommon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestGetMetadataOptions(t *testing.T) {
	options := func(extra ...string) []string {
		return GetMetadata(extra...)[0].Prompt.(*survey.MultiSelect).Options
	}

	assert.NotContains(t, options(), "OriginalEstimate", "create has no handling for it")
	assert.Contains(t, options("OriginalEstimate"), "OriginalEstimate")
}

const fieldsResponse = `[
	{"id":"summary","name":"Summary","custom":false,"schema":{"type":"string"}},
	{"id":"customfield_14694","name":"Platform Component","custom":true,"schema":{"type":"option-with-child"}},
	{"id":"customfield_14695","name":"Environments Affected","custom":true,"schema":{"type":"array","items":"option"}}
]`

func resolveTestClient(t *testing.T, requests *int) *jira.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests++
		assert.Equal(t, "/rest/api/2/field", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fieldsResponse))
	}))
	t.Cleanup(server.Close)

	return jira.NewClient(jira.Config{Server: server.URL}, jira.WithTimeout(3*time.Second))
}

func configureCustomFields(t *testing.T) {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.SetConfigType("yaml")
	require.NoError(t, viper.ReadConfig(strings.NewReader(`
issue:
  fields:
    custom:
      - name: Reason
        key: customfield_12345
        schema:
          datatype: option
      - name: Story Points
        key: customfield_10016
        schema:
          datatype: number
`)))
}

func TestResolveCustomFields(t *testing.T) {
	t.Run("keys that match the config make no request", func(t *testing.T) {
		configureCustomFields(t)

		var requests int
		client := resolveTestClient(t, &requests)

		fields, err := ResolveCustomFields(client, map[string]string{
			"story-points":      "3",
			"Reason":            "x",
			"customfield_12345": "y",
		})

		assert.NoError(t, err)
		assert.Len(t, fields, 2)
		assert.Equal(t, 0, requests)
	})

	t.Run("no custom fields passed makes no request", func(t *testing.T) {
		configureCustomFields(t)

		var requests int
		client := resolveTestClient(t, &requests)

		_, err := ResolveCustomFields(client, nil)

		assert.NoError(t, err)
		assert.Equal(t, 0, requests)
	})

	t.Run("unknown key is looked up on the server once", func(t *testing.T) {
		configureCustomFields(t)

		var requests int
		client := resolveTestClient(t, &requests)

		fields, err := ResolveCustomFields(client, map[string]string{
			"platform-component": "A->B",
			"customfield_14695":  "Development",
			"story-points":       "3",
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, requests)

		byKey := map[string]jira.IssueTypeField{}
		for _, f := range fields {
			byKey[f.Key] = f
		}
		assert.Len(t, byKey, 4)
		assert.Equal(t, "option-with-child", byKey["customfield_14694"].Schema.DataType)
		assert.Equal(t, "Platform Component", byKey["customfield_14694"].Name)
		assert.Equal(t, "array", byKey["customfield_14695"].Schema.DataType)
		assert.Equal(t, "option", byKey["customfield_14695"].Schema.Items)
	})

	t.Run("fields that were not asked for are not added", func(t *testing.T) {
		configureCustomFields(t)

		var requests int
		client := resolveTestClient(t, &requests)

		fields, err := ResolveCustomFields(client, map[string]string{"platform-component": "A"})

		assert.NoError(t, err)
		for _, f := range fields {
			assert.NotEqual(t, "customfield_14695", f.Key)
		}
	})

	t.Run("still unknown keys fail naming all of them", func(t *testing.T) {
		configureCustomFields(t)

		var requests int
		client := resolveTestClient(t, &requests)

		_, err := ResolveCustomFields(client, map[string]string{
			"zzz-typo":           "x",
			"platform-component": "A",
			"aaa-typo":           "y",
		})

		assert.Equal(t, 1, requests)
		assert.ErrorContains(t, err, "aaa-typo, zzz-typo")
		assert.NotContains(t, err.Error(), "platform-component")
		assert.ErrorContains(t, err, "jira issue fields")
		assert.ErrorContains(t, err, "customfield_NNNNN")
	})

	t.Run("non-custom fields from the server are ignored", func(t *testing.T) {
		configureCustomFields(t)

		var requests int
		client := resolveTestClient(t, &requests)

		_, err := ResolveCustomFields(client, map[string]string{"summary": "x"})

		assert.ErrorContains(t, err, "summary")
	})

	t.Run("a failing lookup is reported", func(t *testing.T) {
		configureCustomFields(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		client := jira.NewClient(jira.Config{Server: server.URL}, jira.WithTimeout(3*time.Second))

		_, err := ResolveCustomFields(client, map[string]string{"nope": "x"})

		assert.Error(t, err)
	})
}
