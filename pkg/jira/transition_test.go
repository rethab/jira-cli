package jira

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTransitions(t *testing.T) {
	var (
		apiVersion2          bool
		unexpectedStatusCode bool
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiVersion2 {
			assert.Equal(t, "/rest/api/2/issue/TEST/transitions", r.URL.Path)
		} else {
			assert.Equal(t, "/rest/api/3/issue/TEST/transitions", r.URL.Path)
		}

		assert.Equal(t, "GET", r.Method)

		if unexpectedStatusCode {
			w.WriteHeader(400)
		} else {
			resp, err := os.ReadFile("./testdata/transitions.json")
			assert.NoError(t, err)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write(resp)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.Transitions("TEST")
	assert.NoError(t, err)

	expected := []*Transition{
		{ID: "11", Name: "To Do", IsAvailable: true},
		{ID: "21", Name: "Start work", IsAvailable: true},
		{ID: "31", Name: "Done", IsAvailable: false},
	}
	expected[0].To.ID, expected[0].To.Name = "10000", "To Do"
	expected[1].To.ID, expected[1].To.Name = "10001", "In Progress"
	expected[2].To.ID, expected[2].To.Name = "10002", "Closed"
	assert.Equal(t, expected, actual)

	want, err := os.ReadFile("./testdata/transitions.json")
	assert.NoError(t, err)

	raw, err := client.TransitionsRaw("TEST")
	assert.NoError(t, err)
	assert.Equal(t, string(want), raw)

	apiVersion2 = true

	raw, err = client.TransitionsV2Raw("TEST")
	assert.NoError(t, err)
	assert.Equal(t, string(want), raw)

	unexpectedStatusCode = true

	_, err = client.TransitionsV2("TEST")
	assertUnexpectedResponse(t, err)

	_, err = client.TransitionsV2Raw("TEST")
	assertUnexpectedResponse(t, err)
}

func TestAvailableTransitions(t *testing.T) {
	all := []*Transition{
		{Name: "Start work", IsAvailable: true},
		{Name: "Close", IsAvailable: false},
	}

	t.Run("cloud hides transitions that are not available", func(t *testing.T) {
		assert.Equal(t, []*Transition{all[0]}, AvailableTransitions(all, InstallationTypeCloud))
	})

	t.Run("local keeps all transitions since isAvailable is not returned", func(t *testing.T) {
		assert.Equal(t, all, AvailableTransitions(all, InstallationTypeLocal))
	})
}

func TestTransitionsWithFields(t *testing.T) {
	var apiVersion2 bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiVersion2 {
			assert.Equal(t, "/rest/api/2/issue/TEST/transitions", r.URL.Path)
		} else {
			assert.Equal(t, "/rest/api/3/issue/TEST/transitions", r.URL.Path)
		}
		assert.Equal(t, "transitions.fields", r.URL.Query().Get("expand"))

		resp, err := os.ReadFile("./testdata/transitions_fields.json")
		assert.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(resp)
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	actual, err := client.TransitionsWithFields("TEST")
	assert.NoError(t, err)
	assert.Len(t, actual, 2)

	assert.False(t, actual[0].HasScreen)
	assert.Empty(t, actual[0].Fields)

	rejected := actual[1]
	assert.True(t, rejected.HasScreen)
	assert.Equal(t, "10003", rejected.To.ID)
	assert.Equal(t, "done", rejected.To.StatusCategory.Key)
	assert.Len(t, rejected.Fields, 2)

	resolution := rejected.Fields["resolution"]
	assert.True(t, resolution.Required)
	assert.Equal(t, "Resolution", resolution.Name)
	assert.Equal(t, "resolution", resolution.Schema.System)
	assert.Equal(t, []TransitionFieldValue{{ID: "1", Name: "Done"}, {ID: "2", Name: "Won't Do"}}, resolution.AllowedValues)

	reason := rejected.Fields["customfield_10010"]
	assert.False(t, reason.Required)
	assert.Equal(t, []TransitionFieldValue{{ID: "5", Value: "Duplicate"}}, reason.AllowedValues)

	want, err := os.ReadFile("./testdata/transitions_fields.json")
	assert.NoError(t, err)

	raw, err := client.TransitionsWithFieldsRaw("TEST")
	assert.NoError(t, err)
	assert.Equal(t, string(want), raw)

	apiVersion2 = true

	actual, err = client.TransitionsV2WithFields("TEST")
	assert.NoError(t, err)
	assert.Len(t, actual, 2)

	raw, err = client.TransitionsV2WithFieldsRaw("TEST")
	assert.NoError(t, err)
	assert.Equal(t, string(want), raw)
}

func TestTransition(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/2/issue/TEST/transitions", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		actualBody := new(strings.Builder)
		_, _ = io.Copy(actualBody, r.Body)

		expectedBody := `{"transition":{"id":"31","name":"Done"}}`
		assert.Equal(t, expectedBody, actualBody.String())

		if unexpectedStatusCode {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	requestData := TransitionRequest{Transition: &TransitionRequestData{
		ID:   "31",
		Name: "Done",
	}}
	code, err := client.Transition("TEST", &requestData)
	assert.NoError(t, err)
	assert.Equal(t, 204, code)
}
