package netrc

import (
	"reflect"
	"testing"
)

func TestRead(t *testing.T) {
	netrcOnce.Do(func() {})
	orig := netrc
	t.Cleanup(func() { netrc = orig })

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
			name:    "does not match a different host",
			data:    "machine https://other.atlassian.net login me@sample.org password secret",
			server:  "https://test.atlassian.net",
			wantErr: ErrNetrcEntryNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			netrc = parseNetrc(tt.data)

			got, err := Read(tt.server, "me@sample.org")
			if err != tt.wantErr {
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
