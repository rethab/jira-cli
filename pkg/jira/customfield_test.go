package jira

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCascadingValue(t *testing.T) {
	assert.Equal(t, customFieldTypeOptionWithChild{Value: "Parent"}, parseCascadingValue("Parent"))
	assert.Equal(t, customFieldTypeOptionWithChild{Value: "Parent"}, parseCascadingValue("Parent-> "))
	assert.Equal(t,
		customFieldTypeOptionWithChild{Value: "Parent", Child: &customFieldTypeOption{Value: "A->B"}},
		parseCascadingValue("Parent -> A->B"),
	)
}
