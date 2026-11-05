package move

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rethab/jira-cli/pkg/jira"
)

const statusField = "status"

// hop is one status change on the way to the target.
type hop struct {
	fromID, fromName string
	toID, toName     string
}

// graph holds the status changes seen so far, keyed by status id. Names are only kept for display
// and to order neighbours, since status names can be renamed while ids can't.
type graph struct {
	names map[string]string
	edges map[string]map[string]bool
}

func newGraph() *graph {
	return &graph{names: map[string]string{}, edges: map[string]map[string]bool{}}
}

func (g *graph) addName(id, name string) {
	if id != "" && name != "" && g.names[id] == "" {
		g.names[id] = name
	}
}

func (g *graph) addEdge(fromID, fromName, toID, toName string) {
	if fromID == "" || toID == "" || fromID == toID {
		return
	}

	g.addName(fromID, fromName)
	g.addName(toID, toName)

	if g.edges[fromID] == nil {
		g.edges[fromID] = map[string]bool{}
	}
	g.edges[fromID][toID] = true
}

// addTransitions adds the edges leaving the issue's current status, which are the only ones
// known to be allowed for this issue right now.
func (g *graph) addTransitions(fromID string, transitions []*jira.Transition) {
	for _, t := range transitions {
		g.addEdge(fromID, "", t.To.ID, t.To.Name)
	}
}

// addChangelog learns the edges from status changes that other issues went through.
func (g *graph) addChangelog(histories []jira.ChangelogHistory) {
	for _, h := range histories {
		for _, it := range h.Items {
			if !strings.EqualFold(it.Field, statusField) && !strings.EqualFold(it.FieldID, statusField) {
				continue
			}
			g.addEdge(it.From, it.FromString, it.To, it.ToString)
		}
	}
}

// explore runs a breadth-first search from start and returns the parent of every status reached, plus
// the order in which they were reached. Statuses in done are reached but never left: a global transition
// like "Rejected" would otherwise be the shortest way anywhere, and it sets a resolution on the way.
func (g *graph) explore(start string, done map[string]bool) (map[string]string, []string) {
	parents := map[string]string{start: ""}
	order := []string{start}

	for i := 0; i < len(order); i++ {
		cur := order[i]
		if cur != start && done[cur] {
			continue
		}

		for _, next := range g.sortedNeighbours(cur) {
			if _, seen := parents[next]; seen {
				continue
			}
			parents[next] = cur
			order = append(order, next)
		}
	}

	return parents, order
}

// sortedNeighbours makes the search, and with it the printed path, independent of map iteration order.
func (g *graph) sortedNeighbours(id string) []string {
	out := make([]string, 0, len(g.edges[id]))
	for next := range g.edges[id] {
		out = append(out, next)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := g.names[out[i]], g.names[out[j]]
		if a != b {
			return a < b
		}
		return out[i] < out[j]
	})
	return out
}

// shortestPath returns the fewest hops from start to any of the targets. The bool is false if none is reachable.
func (g *graph) shortestPath(start string, targets, done map[string]bool) ([]hop, bool) {
	parents, order := g.explore(start, done)

	for _, id := range order {
		if id == start || !targets[id] {
			continue
		}

		var hops []hop
		for cur := id; cur != start; cur = parents[cur] {
			p := parents[cur]
			hops = append([]hop{{fromID: p, fromName: g.names[p], toID: cur, toName: g.names[cur]}}, hops...)
		}
		return hops, true
	}

	return nil, false
}

// reachable lists the names of the statuses that can be reached from start.
func (g *graph) reachable(start string, done map[string]bool) []string {
	_, order := g.explore(start, done)

	names := make([]string, 0, len(order))
	for _, id := range order {
		if id != start {
			names = append(names, g.names[id])
		}
	}
	sort.Strings(names)

	return names
}

// requiredFields lists the fields the transition's screen insists on and Jira has no default for.
// It is only meaningful for transitions fetched with their fields expanded.
func requiredFields(t *jira.Transition) []string {
	var ids []string
	for id, f := range t.Fields {
		if f.Required && !f.HasDefaultValue {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	return ids
}

// pickTransition chooses the transition that leads to the next status. Several can lead to the same one, and
// the ones that ask for nothing are the only ones we can take without input, so they win.
func pickTransition(transitions []*jira.Transition, nextID, nextName string) (*jira.Transition, error) {
	var (
		best      *jira.Transition
		bestScore int
	)

	for _, t := range transitions {
		if !leadsTo(t, nextID, nextName) {
			continue
		}

		score := 0
		if t.HasScreen {
			score++
		}
		if len(requiredFields(t)) > 0 {
			score += 2
		}

		if best == nil || score < bestScore {
			best, bestScore = t, score
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no available transition leads to %q", nextName)
	}

	return best, nil
}

func leadsTo(t *jira.Transition, id, name string) bool {
	if id != "" && t.To.ID != "" {
		return t.To.ID == id
	}
	return strings.EqualFold(t.To.Name, name)
}
