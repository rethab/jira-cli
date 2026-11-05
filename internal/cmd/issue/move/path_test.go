package move

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	inTesting   = "1"
	readyToTest = "2"
	codeReview  = "3"
	inProgress  = "4"
	toDo        = "5"
	rejected    = "6"
)

func statusChange(from, fromName, to, toName string) jira.ChangelogHistory {
	return jira.ChangelogHistory{Items: []jira.ChangelogItem{
		{Field: "status", FieldID: "status", From: from, FromString: fromName, To: to, ToString: toName},
	}}
}

func pv2Graph() *graph {
	g := newGraph()
	g.addChangelog([]jira.ChangelogHistory{
		statusChange(readyToTest, "Ready to Test", codeReview, "Code Review"),
		statusChange(codeReview, "Code Review", inProgress, "In Progress"),
		statusChange(inProgress, "In Progress", toDo, "To Do"),
		statusChange(inTesting, "In Testing", rejected, "Rejected"),
		statusChange(rejected, "Rejected", toDo, "To Do"),
	})

	back := &jira.Transition{Name: "Back to Ready to Test"}
	back.To.ID, back.To.Name = readyToTest, "Ready to Test"
	reject := &jira.Transition{Name: "Reject"}
	reject.To.ID, reject.To.Name = rejected, "Rejected"
	g.addTransitions(inTesting, []*jira.Transition{back, reject})

	return g
}

func TestShortestPathAvoidsDoneStatuses(t *testing.T) {
	g := pv2Graph()

	t.Run("routes around the global done status", func(t *testing.T) {
		got, ok := g.shortestPath(inTesting, map[string]bool{toDo: true}, map[string]bool{rejected: true})

		assert.True(t, ok)
		assert.Equal(t, []hop{
			{fromID: inTesting, fromName: "In Testing", toID: readyToTest, toName: "Ready to Test"},
			{fromID: readyToTest, fromName: "Ready to Test", toID: codeReview, toName: "Code Review"},
			{fromID: codeReview, fromName: "Code Review", toID: inProgress, toName: "In Progress"},
			{fromID: inProgress, fromName: "In Progress", toID: toDo, toName: "To Do"},
		}, got)
	})

	t.Run("takes the shortcut without the filter", func(t *testing.T) {
		got, ok := g.shortestPath(inTesting, map[string]bool{toDo: true}, nil)

		assert.True(t, ok)
		assert.Len(t, got, 2)
		assert.Equal(t, "Rejected", got[0].toName)
	})

	t.Run("a done status can still be the target", func(t *testing.T) {
		got, ok := g.shortestPath(inTesting, map[string]bool{rejected: true}, map[string]bool{rejected: true})

		assert.True(t, ok)
		assert.Len(t, got, 1)
	})

	t.Run("a done start can be left", func(t *testing.T) {
		got, ok := g.shortestPath(rejected, map[string]bool{toDo: true}, map[string]bool{rejected: true})

		assert.True(t, ok)
		assert.Len(t, got, 1)
	})

	t.Run("unreachable target", func(t *testing.T) {
		_, ok := g.shortestPath(toDo, map[string]bool{inTesting: true}, nil)

		assert.False(t, ok)
	})

	t.Run("start is not a hop", func(t *testing.T) {
		_, ok := g.shortestPath(toDo, map[string]bool{toDo: true}, nil)

		assert.False(t, ok)
	})
}

func TestShortestPathIsDeterministic(t *testing.T) {
	g := newGraph()
	g.addEdge("1", "Start", "3", "Zeta")
	g.addEdge("1", "Start", "2", "Alpha")
	g.addEdge("2", "Alpha", "9", "End")
	g.addEdge("3", "Zeta", "9", "End")

	for range 20 {
		got, ok := g.shortestPath("1", map[string]bool{"9": true}, nil)

		assert.True(t, ok)
		assert.Equal(t, "Alpha", got[0].toName)
	}
}

func TestReachable(t *testing.T) {
	g := pv2Graph()

	got := g.reachable(inTesting, map[string]bool{rejected: true})

	assert.Equal(t, []string{"Code Review", "In Progress", "Ready to Test", "Rejected", "To Do"}, got)
}

func TestAddChangelogOnlyLearnsStatusChanges(t *testing.T) {
	g := newGraph()
	g.addChangelog([]jira.ChangelogHistory{
		{Items: []jira.ChangelogItem{
			{Field: "assignee", From: "a", To: "b"},
			{Field: "Status", From: "1", FromString: "A", To: "2", ToString: "B"},
			{Field: "resolution", FieldID: "resolution", From: "", To: "10"},
			{Field: "Workflow", FieldID: "status", From: "2", FromString: "B", To: "3", ToString: "C"},
			{Field: "status", From: "", To: "4"},
		}},
	})

	assert.Equal(t, map[string]map[string]bool{"1": {"2": true}, "2": {"3": true}}, g.edges)
	assert.Equal(t, map[string]string{"1": "A", "2": "B", "3": "C"}, g.names)
}

func newTransition(name, toID, toName string) *jira.Transition {
	t := &jira.Transition{Name: name}
	t.To.ID, t.To.Name = toID, toName
	return t
}

func TestPickTransition(t *testing.T) {
	plain := newTransition("Plain", "10", "Target")

	withScreen := newTransition("Screen", "10", "Target")
	withScreen.HasScreen = true

	withRequired := newTransition("Required", "10", "Target")
	withRequired.HasScreen = true
	withRequired.Fields = map[string]jira.TransitionField{"resolution": {Required: true}}

	withDefault := newTransition("Default", "10", "Target")
	withDefault.HasScreen = true
	withDefault.Fields = map[string]jira.TransitionField{"resolution": {Required: true, HasDefaultValue: true}}

	other := newTransition("Other", "11", "Elsewhere")

	t.Run("prefers one without screen or required fields", func(t *testing.T) {
		got, err := pickTransition([]*jira.Transition{withRequired, withScreen, plain, other}, "10", "Target")

		assert.NoError(t, err)
		assert.Equal(t, plain, got)
	})

	t.Run("prefers a screen over required fields", func(t *testing.T) {
		got, err := pickTransition([]*jira.Transition{withRequired, withScreen}, "10", "Target")

		assert.NoError(t, err)
		assert.Equal(t, withScreen, got)
	})

	t.Run("a required field with a default does not count", func(t *testing.T) {
		assert.Empty(t, requiredFields(withDefault))
		assert.Equal(t, []string{"resolution"}, requiredFields(withRequired))
	})

	t.Run("keeps the first of equals", func(t *testing.T) {
		second := newTransition("Second", "10", "Target")

		got, err := pickTransition([]*jira.Transition{plain, second}, "10", "Target")

		assert.NoError(t, err)
		assert.Equal(t, plain, got)
	})

	t.Run("falls back to the status name without ids", func(t *testing.T) {
		noID := newTransition("No id", "", "Target")

		got, err := pickTransition([]*jira.Transition{other, noID}, "10", "target")

		assert.NoError(t, err)
		assert.Equal(t, noID, got)
	})

	t.Run("none matches", func(t *testing.T) {
		_, err := pickTransition([]*jira.Transition{other}, "10", "Target")

		assert.EqualError(t, err, `no available transition leads to "Target"`)
	})
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		params  moveParams
		wantErr string
	}{
		{name: "plain move", params: moveParams{key: "A-1", state: "Done"}},
		{name: "path dry run", params: moveParams{key: "A-1", state: "Done", path: true}},
		{name: "path with yes", params: moveParams{key: "A-1", state: "Done", path: true, yes: true}},
		{name: "yes without path", params: moveParams{key: "A-1", state: "Done", yes: true}, wantErr: "--yes only applies together with --path"},
		{name: "path without state", params: moveParams{key: "A-1", path: true}, wantErr: "--path needs both"},
		{name: "path without key", params: moveParams{state: "Done", path: true}, wantErr: "--path needs both"},
		{name: "path with comment", params: moveParams{key: "A-1", state: "Done", path: true, comment: "x"}, wantErr: "cannot be combined"},
		{name: "path with assignee", params: moveParams{key: "A-1", state: "Done", path: true, assignee: "x"}, wantErr: "cannot be combined"},
		{name: "path with resolution", params: moveParams{key: "A-1", state: "Done", path: true, resolution: "x"}, wantErr: "cannot be combined"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.validatePath()

			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestResolveStatuses(t *testing.T) {
	statuses := []jira.Status{
		{ID: toDo, Name: "To Do"},
		{ID: rejected, Name: "Rejected"},
	}
	statuses[1].StatusCategory.Key = "done"

	cur, targets, done := resolveStatuses(statuses, "to do", "REJECTED")

	assert.Equal(t, toDo, cur)
	assert.Equal(t, map[string]bool{rejected: true}, targets)
	assert.Equal(t, map[string]bool{rejected: true}, done)
}

func TestEscapeJQL(t *testing.T) {
	assert.Equal(t, `Bug \"x\" \\`, escapeJQL(`Bug "x" \`))
}
