package jira

import "strings"

const (
	customFieldFormatOption  = "option"
	customFieldFormatArray   = "array"
	customFieldFormatNumber  = "number"
	customFieldFormatProject = "project"
	// Cascading select.
	customFieldFormatOptionWithChild = "option-with-child"
)

type customField map[string]any

type customFieldTypeNumber float64

type customFieldTypeNumberSet struct {
	Set customFieldTypeNumber `json:"set"`
}

type customFieldTypeStringSet struct {
	Set string `json:"set"`
}

type customFieldTypeOption struct {
	Value string `json:"value"`
}

type customFieldTypeOptionSet struct {
	Set customFieldTypeOption `json:"set"`
}

type customFieldTypeOptionAddRemove struct {
	Add    *customFieldTypeOption `json:"add,omitempty"`
	Remove *customFieldTypeOption `json:"remove,omitempty"`
}

type customFieldTypeOptionWithChild struct {
	Value string                 `json:"value"`
	Child *customFieldTypeOption `json:"child,omitempty"`
}

type customFieldTypeOptionWithChildSet struct {
	Set customFieldTypeOptionWithChild `json:"set"`
}

const cascadingSeparator = "->"

// parseCascadingValue splits "Parent->Child" on the first separator only, since option
// values are free text and a child may legitimately contain the separator.
func parseCascadingValue(val string) customFieldTypeOptionWithChild {
	parent, child, found := strings.Cut(val, cascadingSeparator)

	out := customFieldTypeOptionWithChild{Value: strings.TrimSpace(parent)}
	if found {
		out.Child = &customFieldTypeOption{Value: strings.TrimSpace(child)}
	}
	return out
}

type customFieldTypeProject struct {
	Value string `json:"key"`
}

type customFieldTypeProjectSet struct {
	Set customFieldTypeProject `json:"set"`
}

type customFieldTypeArraySet struct {
	Set []string `json:"set"`
}
