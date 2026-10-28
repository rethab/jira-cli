package jira

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEditCustomFieldArray(t *testing.T) {
	expectedBody := `{"update":{"customfield_10001":[{"set":["a","b","c"]}]},"fields":{"parent":{}}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/2/issue/TEST-1", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)

		actualBody := new(strings.Builder)
		_, _ = io.Copy(actualBody, r.Body)

		assert.JSONEq(t, expectedBody, actualBody.String())

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	req := &EditRequest{
		CustomFields: map[string]string{
			"multi-select": "a,b,c",
		},
	}
	req.WithCustomFields([]IssueTypeField{
		{
			Name: "Multi Select",
			Key:  "customfield_10001",
			Schema: struct {
				DataType string `json:"type"`
				Items    string `json:"items,omitempty"`
			}{DataType: customFieldFormatArray, Items: "string"},
		},
	})

	err := client.Edit("TEST-1", req)
	assert.NoError(t, err)
}

func TestConstructCustomFieldsForEditArrayOfStrings(t *testing.T) {
	data := &editRequest{}

	configuredFields := []IssueTypeField{
		{
			Name: "Multi Select",
			Key:  "customfield_10001",
			Schema: struct {
				DataType string `json:"type"`
				Items    string `json:"items,omitempty"`
			}{DataType: customFieldFormatArray, Items: "string"},
		},
	}
	fields := map[string]string{
		"multi-select": "a,b,c",
	}

	constructCustomFieldsForEdit(fields, configuredFields, data)

	expected := []customFieldTypeArraySet{{Set: []string{"a", "b", "c"}}}
	assert.Equal(t, expected, data.Update.M.customFields["customfield_10001"])
}

func TestEditOriginalEstimate(t *testing.T) {
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		gotBody = string(body)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	err := client.Edit("TEST-1", &EditRequest{OriginalEstimate: "1d 4h"})
	assert.NoError(t, err)
	assert.Contains(t, gotBody, `"timetracking":[{"edit":{"originalEstimate":"1d 4h"}}]`)
	assert.NotContains(t, gotBody, "remainingEstimate")

	err = client.Edit("TEST-1", &EditRequest{Summary: "New summary"})
	assert.NoError(t, err)
	assert.NotContains(t, gotBody, "timetracking")
}

func TestEdit(t *testing.T) {
	var gotRawQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/2/issue/TEST-1", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)

		gotRawQuery = r.URL.RawQuery

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	err := client.Edit("TEST-1", &EditRequest{ParentIssueKey: "EPIC-1"})
	assert.NoError(t, err)
	assert.Empty(t, gotRawQuery, "notifyUsers should not be set when SkipNotify is not requested")

	err = client.Edit("TEST-1", &EditRequest{ParentIssueKey: "EPIC-1", SkipNotify: true})
	assert.NoError(t, err)
	assert.Equal(t, "notifyUsers=false", gotRawQuery, "notifyUsers=false should only be set when SkipNotify is requested")
}

func TestEditCascadingSelectCustomField(t *testing.T) {
	component := IssueTypeField{Name: "Platform Component", Key: "customfield_14694"}
	component.Schema.DataType = customFieldFormatOptionWithChild

	cases := []struct {
		name  string
		value string
		set   string
	}{
		{
			name:  "parent and child",
			value: "Pipelines/Scripts -> Environment Management",
			set:   `{"value":"Pipelines/Scripts","child":{"value":"Environment Management"}}`,
		},
		{name: "parent only", value: "Pipelines/Scripts", set: `{"value":"Pipelines/Scripts"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectedBody := `{"update":{"customfield_14694":[{"set":` + tc.set + `}]},"fields":{"parent":{}}}`

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				actualBody := new(strings.Builder)
				_, _ = io.Copy(actualBody, r.Body)

				assert.JSONEq(t, expectedBody, actualBody.String())

				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

			req := &EditRequest{CustomFields: map[string]string{"customfield_14694": tc.value}}
			req.WithCustomFields([]IssueTypeField{component})

			assert.NoError(t, client.Edit("TEST-1", req))
		})
	}
}
