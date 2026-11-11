package cmdcommon

import (
	"errors"
	"strings"
)

// ParseReplace parses a replace flag value in the format <search>:<replace>.
// The value is split on the first colon only, so the replacement may contain
// colons (eg: URLs) but the search string may not. An empty search string is
// rejected as it would match between every character.
func ParseReplace(r string) (string, string, error) {
	from, to, found := strings.Cut(r, ":")
	if !found {
		return "", "", errors.New("must be in format <search>:<replace>")
	}
	if from == "" {
		return "", "", errors.New("search string cannot be empty")
	}
	return from, to, nil
}
