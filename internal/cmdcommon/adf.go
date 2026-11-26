package cmdcommon

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/adf"
)

// ReadADFDocument reads the ADF document passed to flag from path, or standard input for -, and parses it
// with ParseADFDocument.
func ReadADFDocument(path, flag, source string) (json.RawMessage, *adf.ADF, error) {
	b, err := cmdutil.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read %s: %w", flag, err)
	}
	return ParseADFDocument(b, flag, source)
}

// ParseADFDocument checks the root of an ADF document passed to flag and returns the bytes to send unchanged.
// source says where the document comes from, eg ".fields.description of jira issue view --raw", for the hint
// given when a wrapper around the document was passed instead.
//
// Only the root is checked; anything deeper is left to Jira, which validates the document. The returned bytes
// are what gets sent: adf.ADF models only part of what Jira accepts, so writing it back out would silently drop
// the rest.
func ParseADFDocument(b []byte, flag, source string) (json.RawMessage, *adf.ADF, error) {
	// Some Windows editors and PowerShell redirection write a byte order mark, which
	// json.Valid rejects.
	b = bytes.TrimSpace(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")))
	if len(b) == 0 {
		return nil, nil, fmt.Errorf("%s is empty", flag)
	}
	if !json.Valid(b) {
		var syntax any
		return nil, nil, fmt.Errorf("%s is not valid JSON: %w", flag, json.Unmarshal(b, &syntax))
	}

	notADoc := fmt.Errorf(`%s must be an Atlassian document: a JSON object with "type": "doc", "version": 1 and a "content" array`, flag)
	wrapped := fmt.Errorf("%s takes the document itself, not what surrounds it; pass %s", flag, source)

	if b[0] == '[' {
		return nil, nil, wrapped
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(b, &root); err != nil || root == nil {
		return nil, nil, notADoc
	}
	_, hasType := root["type"]
	_, hasBody := root["body"]
	if _, hasFields := root["fields"]; hasFields || (hasBody && !hasType) {
		return nil, nil, wrapped
	}
	if !bytes.HasPrefix(bytes.TrimSpace(root["content"]), []byte("[")) {
		return nil, nil, notADoc
	}

	var doc adf.ADF
	if err := json.Unmarshal(b, &doc); err != nil || doc.DocType != "doc" || doc.Version != 1 {
		return nil, nil, notADoc
	}

	return json.RawMessage(b), &doc, nil
}
