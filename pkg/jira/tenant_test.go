package jira

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFetchCloudID(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr string
	}{
		{name: "success", status: http.StatusOK, body: `{"cloudId":"abc-123"}`, want: "abc-123"},
		{name: "non-200", status: http.StatusNotFound, body: `not found`, wantErr: "404"},
		{name: "malformed json", status: http.StatusOK, body: `<html>`, wantErr: "decoding"},
		{name: "missing cloud id", status: http.StatusOK, body: `{}`, wantErr: "no cloud ID"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/_edge/tenant_info", r.URL.Path)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			// The trailing slash must not produce a double slash in the request path.
			got, err := FetchCloudID(&http.Client{Timeout: 3 * time.Second}, srv.URL+"/")
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
