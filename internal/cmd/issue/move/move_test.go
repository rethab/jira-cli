package move

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

func TestVerifyTransitionErrorListsTargetStatuses(t *testing.T) {
	start := &jira.Transition{Name: "Start work", IsAvailable: true}
	start.To.Name = "In Progress"
	noTarget := &jira.Transition{Name: "Reopen", IsAvailable: true}

	mc := moveCmd{
		transitions: []*jira.Transition{start, noTarget},
		params:      &moveParams{key: "TEST-1", state: "Nope"},
	}

	_, err := mc.verifyTransition(jira.InstallationTypeCloud)

	assert.EqualError(t, err,
		"invalid transition state \"Nope\"\nAvailable states for issue TEST-1: 'Start work' -> 'In Progress', 'Reopen'")
}
