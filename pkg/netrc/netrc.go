// Package netrc implements GNU .netrc specification.
// This implementation is borrowed from the original implementation by the go authors.
// See https://github.com/golang/go/blob/master/src/cmd/go/internal/auth/netrc.go
// See https://www.gnu.org/software/inetutils/manual/html_node/The-_002enetrc-file.html
package netrc

import (
	"fmt"
	"net/url"
	"strings"
)

// ErrNetrcEntryNotFound is thrown if details for the machine is not found.
var ErrNetrcEntryNotFound = fmt.Errorf("netrc config: entry not found")

// Entry is a netrc config entry.
type Entry struct {
	Machine  string
	Login    string
	Password string
}

// Read reads config for the given machine.
func Read(machine string, login string) (*Entry, error) {
	netrcOnce.Do(readNetrc)
	if netrcErr != nil {
		return nil, netrcErr
	}

	serverURL, err := url.ParseRequestURI(machine)
	if err != nil {
		return nil, err
	}

	// The netrc spec has no ports, so a bare host must also match a server URL
	// that carries one. An entry naming the exact host:port still wins, so
	// adding the fallback never changes which entry an existing setup resolves to.
	for _, host := range []string{serverURL.Host, serverURL.Hostname()} {
		for _, line := range netrc {
			if strings.EqualFold(machineHost(line.machine), host) && line.login == login {
				return &Entry{
					Machine:  line.machine,
					Login:    line.login,
					Password: line.password,
				}, nil
			}
		}
	}

	return nil, ErrNetrcEntryNotFound
}

// machineHost returns the host of a netrc machine field. The spec calls for a
// bare host, but users commonly write the full server URL.
func machineHost(machine string) string {
	if !strings.Contains(machine, "://") {
		return machine
	}
	u, err := url.Parse(machine)
	if err != nil || u.Host == "" {
		return machine
	}
	return u.Host
}
