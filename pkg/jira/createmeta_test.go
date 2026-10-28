package jira

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetCreateMeta(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/2/issue/createmeta", r.URL.Path)

		if unexpectedStatusCode {
			w.WriteHeader(400)
		} else {
			assert.Equal(t, url.Values{
				"projectKeys":    []string{"TEST"},
				"issuetypeNames": []string{"Epic"},
				"expand":         []string{"projects.issuetypes.fields"},
			}, r.URL.Query())

			resp, err := os.ReadFile("./testdata/createmeta.json")
			assert.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write(resp)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.GetCreateMeta(&CreateMetaRequest{
		Projects:       "TEST",
		IssueTypeNames: "Epic",
		Expand:         "projects.issuetypes.fields",
	})
	assert.NoError(t, err)

	expected := &CreateMetaResponse{[]struct {
		Key        string                 `json:"key"`
		Name       string                 `json:"name"`
		IssueTypes []*CreateMetaIssueType `json:"issuetypes"`
	}{
		{
			Key:  "TEST",
			Name: "Test Project",
			IssueTypes: []*CreateMetaIssueType{
				{
					IssueType: IssueType{
						ID:      "10001",
						Name:    "Epic",
						Subtask: false,
					},
					Fields: map[string]IssueTypeField{
						"customfield_10011": {
							Name: "Epic Name",
							Key:  "customfield_10011",
						},
						"priority": {
							Name: "Priority",
							Key:  "priority",
						},
						"customfield_10014": {
							Name: "Epic Link",
							Key:  "customfield_10014",
						},
					},
				},
			},
		},
	}}
	assert.Equal(t, expected, actual)

	unexpectedStatusCode = true

	_, err = client.GetCreateMeta(&CreateMetaRequest{
		Projects:       "TEST",
		IssueTypeNames: "Epic",
		Expand:         "projects.issuetypes.fields",
	})
	assertUnexpectedResponse(t, err)
}

func TestGetCreateMetaForJiraServerV9(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/2/issue/createmeta/TEST/issuetypes", r.URL.Path)

		if unexpectedStatusCode {
			w.WriteHeader(400)
		} else {
			assert.Equal(t, url.Values{
				"issuetypeNames": []string{"Epic"},
				"expand":         []string{"projects.issuetypes.fields"},
			}, r.URL.Query())

			resp, err := os.ReadFile("./testdata/createmetav9.json")
			assert.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write(resp)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.GetCreateMetaForJiraServerV9(&CreateMetaRequest{
		Projects:       "TEST",
		IssueTypeNames: "Epic",
		Expand:         "projects.issuetypes.fields",
	})
	assert.NoError(t, err)

	expected := &CreateMetaResponseJiraServerV9{[]struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Subtask bool   `json:"subtask"`
	}{
		{
			ID:      "10001",
			Name:    "Epic",
			Subtask: false,
		},
		{
			ID:      "10002",
			Name:    "Task",
			Subtask: false,
		},
		{
			ID:      "10003",
			Name:    "Sub-task",
			Subtask: true,
		},
	}}
	assert.Equal(t, expected, actual)

	unexpectedStatusCode = true

	_, err = client.GetCreateMetaForJiraServerV9(&CreateMetaRequest{
		Projects: "TEST",
		Expand:   "projects.issuetypes.fields",
	})
	assertUnexpectedResponse(t, err)
}

const cloudIssueTypesPage1 = `{"startAt":0,"maxResults":2,"total":3,"issueTypes":[
	{"id":"10001","name":"Epic","subtask":false},
	{"id":"10002","name":"Problem","subtask":false}]}`

const cloudIssueTypesPage2 = `{"startAt":2,"maxResults":2,"total":3,"issueTypes":[
	{"id":"10003","name":"Sub-task","subtask":true}]}`

const cloudFieldsPage1 = `{"startAt":0,"maxResults":2,"total":3,"fields":[
	{"fieldId":"summary","key":"summary","name":"Summary","required":true,"schema":{"type":"string","system":"summary"}},
	{"fieldId":"priority","key":"priority","name":"Priority","required":false,"schema":{"type":"priority"},
	 "allowedValues":[{"id":"1","name":"High"},{"id":"2","name":"Low"}]}]}`

const cloudFieldsPage2 = `{"startAt":2,"maxResults":2,"total":3,"fields":[
	{"fieldId":"customfield_14694","key":"customfield_14694","name":"Platform Component","required":true,
	 "schema":{"type":"option-with-child","custom":"com.atlassian.jira.plugin.system.customfieldtypes:cascadingselect"},
	 "allowedValues":[{"id":"1","value":"Pipelines/Scripts","children":[{"id":"2","value":"Environment Management"}]}]}]}`

func serveJSONPages(t *testing.T, path string, pages map[string]string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path, r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		page, ok := pages[r.URL.Query().Get("startAt")]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(page))
	}))
}

func TestGetCreateMetaIssueTypesCloudPagination(t *testing.T) {
	server := serveJSONPages(t, "/rest/api/2/issue/createmeta/TEST/issuetypes", map[string]string{
		"0": cloudIssueTypesPage1,
		"2": cloudIssueTypesPage2,
	})
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.GetCreateMetaIssueTypes("TEST")
	assert.NoError(t, err)

	assert.Equal(t, []IssueType{
		{ID: "10001", Name: "Epic"},
		{ID: "10002", Name: "Problem"},
		{ID: "10003", Name: "Sub-task", Subtask: true},
	}, actual)
}

func TestGetCreateMetaIssueTypesServerValues(t *testing.T) {
	server := serveJSONPages(t, "/rest/api/2/issue/createmeta/TEST/issuetypes", map[string]string{
		"0": `{"startAt":0,"maxResults":50,"total":1,"isLastPage":true,"values":[{"id":"10001","name":"Epic"}]}`,
	})
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.GetCreateMetaIssueTypes("TEST")
	assert.NoError(t, err)
	assert.Equal(t, []IssueType{{ID: "10001", Name: "Epic"}}, actual)
}

func TestGetCreateMetaFieldsCloudPagination(t *testing.T) {
	server := serveJSONPages(t, "/rest/api/2/issue/createmeta/TEST/issuetypes/10002", map[string]string{
		"0": cloudFieldsPage1,
		"2": cloudFieldsPage2,
	})
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.GetCreateMetaFields("TEST", "10002")
	assert.NoError(t, err)

	assert.Len(t, actual, 3)

	assert.Equal(t, "summary", actual[0].ID())
	assert.Equal(t, "Summary", actual[0].Name)
	assert.True(t, actual[0].Required)
	assert.Equal(t, "string", actual[0].Schema.DataType)
	assert.Empty(t, actual[0].AllowedValues)

	assert.False(t, actual[1].Required)
	assert.Equal(t, []CreateMetaAllowedValue{{Name: "High"}, {Name: "Low"}}, actual[1].AllowedValues)

	assert.Equal(t, "customfield_14694", actual[2].ID())
	assert.Equal(t, "option-with-child", actual[2].Schema.DataType)
	assert.Equal(t, []CreateMetaAllowedValue{{
		Value:    "Pipelines/Scripts",
		Children: []CreateMetaAllowedValue{{Value: "Environment Management"}},
	}}, actual[2].AllowedValues)
}

func TestGetCreateMetaFieldsServerValues(t *testing.T) {
	server := serveJSONPages(t, "/rest/api/2/issue/createmeta/TEST/issuetypes/10002", map[string]string{
		"0": `{"startAt":0,"maxResults":50,"total":1,"isLastPage":true,"values":[
			{"fieldId":"customfield_10001","name":"Reason","required":true,"schema":{"type":"array","items":"option"},
			 "allowedValues":[{"value":"A"}]}]}`,
	})
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.GetCreateMetaFields("TEST", "10002")
	assert.NoError(t, err)

	assert.Len(t, actual, 1)
	assert.Equal(t, "customfield_10001", actual[0].ID())
	assert.Equal(t, "array", actual[0].Schema.DataType)
	assert.Equal(t, "option", actual[0].Schema.Items)
}

func TestGetCreateMetaFieldsUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	_, err := client.GetCreateMetaFields("TEST", "10002")
	assertUnexpectedResponse(t, err)

	_, err = client.GetCreateMetaIssueTypes("TEST")
	assertUnexpectedResponse(t, err)
}

const legacyCreateMeta = `{"projects":[{"key":"TEST","name":"Test","issuetypes":[
	{"id":"10001","name":"Epic","subtask":false,"fields":{}},
	{"id":"10002","name":"Problem","subtask":false,"fields":{
		"summary":{"required":true,"name":"Summary","key":"summary","schema":{"type":"string"}},
		"customfield_10001":{"required":false,"name":"Reason","schema":{"type":"option"},
			"allowedValues":[{"value":"A"},{"value":"B"}]}}}]}]}`

func serveLegacyCreateMeta(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/2/issue/createmeta", r.URL.Path)
		assert.Equal(t, "TEST", r.URL.Query().Get("projectKeys"))
		assert.Equal(t, "projects.issuetypes.fields", r.URL.Query().Get("expand"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(legacyCreateMeta))
	}))
}

func TestGetCreateFieldsLegacy(t *testing.T) {
	server := serveLegacyCreateMeta(t)
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.GetCreateFields("TEST", "problem", true)
	assert.NoError(t, err)

	assert.Len(t, actual, 2)
	// Sorted by id since the server returns a map.
	assert.Equal(t, "customfield_10001", actual[0].ID())
	assert.Equal(t, "Reason", actual[0].Name)
	assert.Equal(t, []CreateMetaAllowedValue{{Value: "A"}, {Value: "B"}}, actual[0].AllowedValues)
	assert.Equal(t, "summary", actual[1].ID())
	assert.True(t, actual[1].Required)
}

func TestGetCreateFieldsLegacyUnknownIssueType(t *testing.T) {
	server := serveLegacyCreateMeta(t)
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	_, err := client.GetCreateFields("TEST", "Nope", true)
	assert.EqualError(t, err, `issue type "Nope" not found in project TEST, valid types: Epic, Problem`)
}

func TestGetCreateFields(t *testing.T) {
	var fieldRequests int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/rest/api/2/issue/createmeta/TEST/issuetypes":
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":2,"issueTypes":[
				{"id":"10001","name":"Epic"},{"id":"10002","name":"Problem"}]}`))
		case "/rest/api/2/issue/createmeta/TEST/issuetypes/10002":
			fieldRequests++
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":1,"fields":[
				{"fieldId":"summary","name":"Summary","required":true,"schema":{"type":"string"}}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	t.Run("issue type is matched case-insensitively and resolved to its id", func(t *testing.T) {
		actual, err := client.GetCreateFields("TEST", "pROBLEM", false)
		assert.NoError(t, err)
		assert.Len(t, actual, 1)
		assert.Equal(t, 1, fieldRequests)
	})

	t.Run("unknown issue type lists the valid ones", func(t *testing.T) {
		_, err := client.GetCreateFields("TEST", "Nope", false)
		assert.EqualError(t, err, `issue type "Nope" not found in project TEST, valid types: Epic, Problem`)
	})
}
