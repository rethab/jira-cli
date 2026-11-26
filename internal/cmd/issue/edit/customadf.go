package edit

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rethab/jira-cli/internal/cmdcommon"
	"github.com/rethab/jira-cli/pkg/jira"
)

const stdinReaders = "only one of --template, --adf-body and --custom-adf can read standard input"

// customADFValue is one <field>=<path|-> of --custom-adf.
type customADFValue struct {
	key, path string
	doc       json.RawMessage
}

// StringArray with a split on the first equals sign rather than StringToString, whose CSV
// parsing would break on a comma in a path.
func parseCustomADF(values []string) ([]customADFValue, error) {
	var out []customADFValue
	for _, v := range values {
		key, path, ok := strings.Cut(v, "=")
		if !ok || key == "" || path == "" {
			return nil, fmt.Errorf("invalid --custom-adf %q: want <field>=<path|->", v)
		}
		out = append(out, customADFValue{key: key, path: path})
	}
	return out, nil
}

func checkCustomADF(params *editParams, local bool) error {
	if len(params.customADF) == 0 {
		return nil
	}
	if local {
		return errors.New("--custom-adf needs Jira Cloud; on Jira Server and Data Center, --custom writes " +
			"wiki markup as is")
	}

	for i, v := range params.customADF {
		for _, earlier := range params.customADF[:i] {
			if strings.EqualFold(v.key, earlier.key) {
				return fmt.Errorf("--custom-adf is given %q more than once", v.key)
			}
		}
		for key := range params.customFields {
			if strings.EqualFold(v.key, key) {
				return fmt.Errorf("%q is given to both --custom and --custom-adf", v.key)
			}
		}
	}

	// An explicit --body wins over the template, in which case stdin is left alone.
	readers := 0
	if params.template == "-" && params.body == "" {
		readers++
	}
	if params.adfBody == "-" {
		readers++
	}
	if readers+customADFStdinReaders(params.customADF) > 1 {
		return errors.New(stdinReaders)
	}

	return nil
}

func customADFStdinReaders(values []customADFValue) int {
	n := 0
	for _, v := range values {
		if v.path == "-" {
			n++
		}
	}
	return n
}

// Like loadADFBody, this runs before the issue is fetched so that a bad file fails
// with nothing written.
func loadCustomADF(params *editParams) error {
	for i, v := range params.customADF {
		var err error
		params.customADF[i].doc, _, err = cmdcommon.ReadADFDocument(
			v.path, "--custom-adf "+v.key, ".fields.<field id> of jira issue view --raw")
		if err != nil {
			return err
		}
	}
	return nil
}

func customADFKeys(values []customADFValue) map[string]string {
	keys := make(map[string]string, len(values))
	for _, v := range values {
		keys[v.key] = ""
	}
	return keys
}

// matchingFieldIDs is a set in the order the fields were found, since a field can
// be listed more than once when it is configured and also looked up.
func matchingFieldIDs(key string, fields []jira.IssueTypeField) []string {
	var ids []string
	for _, f := range fields {
		if jira.CustomFieldMatches(key, f) && !slices.Contains(ids, f.Key) {
			ids = append(ids, f.Key)
		}
	}
	return ids
}

// Keys can be names or ids, so two of them, or one of them and a --custom key, can
// name the same field; sending both would make one silently win.
func resolveCustomADF(values []customADFValue, custom map[string]string, fields []jira.IssueTypeField) (map[string]json.RawMessage, error) {
	resolved := make(map[string]json.RawMessage, len(values))
	keyOf := make(map[string]string, len(values))

	for _, v := range values {
		ids := matchingFieldIDs(v.key, fields)
		switch len(ids) {
		case 0:
			return nil, fmt.Errorf("--custom-adf %q matches no custom field", v.key)
		case 1:
		default:
			return nil, fmt.Errorf("--custom-adf %q matches more than one field; pass the field id instead", v.key)
		}

		if other, ok := keyOf[ids[0]]; ok {
			return nil, fmt.Errorf("--custom-adf %q and %q are the same field", v.key, other)
		}
		keyOf[ids[0]] = v.key
		resolved[ids[0]] = v.doc
	}

	customKeys := make([]string, 0, len(custom))
	for key := range custom {
		customKeys = append(customKeys, key)
	}
	slices.Sort(customKeys)

	for _, key := range customKeys {
		for _, id := range matchingFieldIDs(key, fields) {
			if adfKey, ok := keyOf[id]; ok {
				return nil, fmt.Errorf("%q of --custom and %q of --custom-adf are the same field", key, adfKey)
			}
		}
	}

	return resolved, nil
}

// The description of --adf-body goes in the same request, so that watchers hear of the
// change once. The v2 edit request can't carry ADF, which is why these fields have a
// request of their own.
func adfFieldsWrite(client *jira.Client, params *editParams) func(skipNotify bool) error {
	if params.adfDoc == nil && len(params.customADFFields) == 0 {
		return nil
	}

	fields := make(map[string]json.RawMessage, len(params.customADFFields)+1)
	for id, doc := range params.customADFFields {
		fields[id] = doc
	}
	if params.adfDoc != nil {
		fields["description"] = params.adfDoc
	}

	return func(skipNotify bool) error { return client.SetIssueFieldsADF(params.issueKey, fields, skipNotify) }
}
