package jira

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatuses(t *testing.T) {
	var unexpectedStatusCode bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/rest/api/2/status", r.URL.Path)

		if unexpectedStatusCode {
			w.WriteHeader(400)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[
			{"id":"1","name":"To Do","statusCategory":{"key":"new"}},
			{"id":"2","name":"Done","statusCategory":{"key":"done"}}]`))
	}))
	defer server.Close()

	client := NewClient(Config{Server: server.URL}, WithTimeout(3*time.Second))

	out, err := client.Statuses()
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "To Do", out[0].Name)
	assert.Equal(t, "done", out[1].StatusCategory.Key)

	unexpectedStatusCode = true

	_, err = client.Statuses()
	assertUnexpectedResponse(t, err)
}
