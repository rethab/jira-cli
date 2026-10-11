package netrc

import (
	"errors"
	"reflect"
	"testing"
)

func TestRead(t *testing.T) {
	netrcOnce.Do(func() {})
	orig, origErr := netrc, netrcErr
	netrcErr = nil
	t.Cleanup(func() { netrc, netrcErr = orig, origErr })

	tests := []struct {
		name    string
		data    string
		server  string
		wantErr error
	}{
		{
			name:   "matches bare host machine",
			data:   "machine test.atlassian.net login me@sample.org password secret",
			server: "https://test.atlassian.net",
		},
		{
			name:   "matches full URL machine",
			data:   "machine https://test.atlassian.net login me@sample.org password secret",
			server: "https://test.atlassian.net",
		},
		{
			name:   "matches full URL machine with trailing slash",
			data:   "machine https://test.atlassian.net/ login me@sample.org password secret",
			server: "https://test.atlassian.net",
		},
		{
			name:   "matches host case-insensitively",
			data:   "machine https://Test.Atlassian.net login me@sample.org password secret",
			server: "https://test.atlassian.net",
		},
		{
			name:   "matches bare host against server with port",
			data:   "machine jira.example.com login me@sample.org password secret",
			server: "https://jira.example.com:8443",
		},
		{
			name:   "matches host and port machine",
			data:   "machine jira.example.com:8443 login me@sample.org password secret",
			server: "https://jira.example.com:8443",
		},
		{
			name:   "matches full URL machine with port",
			data:   "machine https://jira.example.com:8443/jira login me@sample.org password secret",
			server: "https://jira.example.com:8443/jira",
		},
		{
			name: "prefers exact host and port over bare host",
			data: "machine jira.example.com login me@sample.org password other\n" +
				"machine jira.example.com:8443 login me@sample.org password secret",
			server: "https://jira.example.com:8443",
		},
		{
			name:    "does not match a different port",
			data:    "machine https://jira.example.com:9000 login me@sample.org password secret",
			server:  "https://jira.example.com:8443",
			wantErr: ErrNetrcEntryNotFound,
		},
		{
			name:    "does not match a different host",
			data:    "machine https://other.atlassian.net login me@sample.org password secret",
			server:  "https://test.atlassian.net",
			wantErr: ErrNetrcEntryNotFound,
		},
		{
			name:    "does not match a different login",
			data:    "machine test.atlassian.net login other@sample.org password secret",
			server:  "https://test.atlassian.net",
			wantErr: ErrNetrcEntryNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			netrc = parseNetrc(tt.data)

			got, err := Read(tt.server, "me@sample.org")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Read() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && got.Password != "secret" {
				t.Errorf("Read() password = %q, want %q", got.Password, "secret")
			}
		})
	}
}

func Test_parseNetrc(t *testing.T) {
	type args struct {
		data string
	}
	tests := []struct {
		name string
		args args
		want []netrcLine
	}{
		{
			name: "parses netrc file properly",
			args: args{
				data: "machine test.sample.org\nlogin mylogin@sample.org\npassword mypassword",
			},
			want: []netrcLine{
				{
					machine:  "test.sample.org",
					login:    "mylogin@sample.org",
					password: "mypassword",
				},
			},
		},
		{
			name: "does not parse incomplete netrc",
			args: args{
				data: "machine test.sample.org\npassword mypassword",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseNetrc(tt.args.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseNetrc() = %v, want %v", got, tt.want)
			}
		})
	}
}
