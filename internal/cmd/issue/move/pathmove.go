package move

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/viper"

	"github.com/rethab/jira-cli/api"
	"github.com/rethab/jira-cli/internal/cmdutil"
	"github.com/rethab/jira-cli/pkg/jira"
)

const (
	// Enough recent issues to see every common route without turning a move into hundreds of requests.
	sampleSize    = 50
	sampleWorkers = 5

	doneCategory = "done"
)

// pathIssue is the part of an issue --path needs. The status is read by id since its name is not unique.
type pathIssue struct {
	Fields struct {
		Project struct {
			Key string `json:"key"`
		} `json:"project"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Status struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

func (p *moveParams) validatePath() error {
	switch {
	case p.yes && !p.path:
		return errors.New("--yes only applies together with --path")
	case !p.path:
		return nil
	case p.key == "" || p.state == "":
		return errors.New("--path needs both ISSUE-KEY and the target status, eg: jira issue move ISSUE-1 \"To Do\" --path")
	case p.comment != "" || p.assignee != "" || p.resolution != "":
		return errors.New("--path cannot be combined with --comment, --assignee or --resolution")
	}
	return nil
}

func (mc *moveCmd) movePath(installation string, web bool) error {
	p := mc.params

	raw, err := withSpinner("Fetching issue...", func() (string, error) {
		return api.ProxyGetIssueRaw(mc.client, p.key)
	})
	if err != nil {
		return err
	}

	var issue pathIssue
	if err := json.Unmarshal([]byte(raw), &issue); err != nil {
		return err
	}

	project, issueType := issue.Fields.Project.Key, issue.Fields.IssueType.Name
	currentID, currentName := issue.Fields.Status.ID, issue.Fields.Status.Name
	if project == "" || issueType == "" || currentID == "" {
		return fmt.Errorf("could not read the project, issue type and status of %s", p.key)
	}

	statuses, err := withSpinner("Fetching statuses...", mc.client.Statuses)
	if err != nil {
		return err
	}

	targets, done := resolveStatuses(statuses, p.state)
	if len(targets) == 0 {
		return fmt.Errorf("unknown status %q", p.state)
	}
	if targets[currentID] {
		fmt.Printf("Issue %s is already in status %q\n", p.key, currentName)
		return nil
	}

	// With fields, so the dry run picks the same first transition as the walk and can tell what it requires.
	available, err := withSpinner("Fetching available transitions...", func() ([]*jira.Transition, error) {
		ts, err := api.ProxyTransitionsWithFields(mc.client, p.key)
		return jira.AvailableTransitions(ts, installation), err
	})
	if err != nil {
		return err
	}

	g := newGraph()
	for _, s := range statuses {
		g.addName(s.ID, s.Name)
	}
	g.addName(currentID, currentName)

	sampled, err := withSpinner("Learning the workflow from other issues...", func() (int, error) {
		return mc.learn(g, project, issueType)
	})
	if err != nil {
		return err
	}
	g.setTransitions(currentID, available)

	hops, ok := g.shortestPath(currentID, targets, done)
	if !ok {
		return fmt.Errorf(
			"no path from %q to %q found in the history of %d %s %s issues\nStatuses reachable from %q: %s",
			currentName, p.state, sampled, project, issueType, currentName,
			strings.Join(g.reachable(currentID, done), ", "),
		)
	}

	if !p.yes {
		printPath(hops, available, project, issueType, sampled)
		return nil
	}

	if err := mc.walk(hops, installation); err != nil {
		return err
	}

	server := viper.GetString("server")
	fmt.Printf("%s\n", cmdutil.GenerateServerBrowseURL(server, p.key))

	if web {
		return cmdutil.Navigate(server, p.key)
	}
	return nil
}

// learn adds what the changelogs of the issue itself and of recent issues of the same project and type
// say about the workflow, and returns how many other issues it looked at.
func (mc *moveCmd) learn(g *graph, project, issueType string) (int, error) {
	jql := fmt.Sprintf(
		`project = "%s" AND issuetype = "%s" AND key != "%s" ORDER BY updated DESC`,
		escapeJQL(project), escapeJQL(issueType), escapeJQL(mc.params.key),
	)

	res, err := api.ProxySearch(mc.client, jql, 0, sampleSize)
	if err != nil {
		return 0, err
	}

	keys := []string{mc.params.key}
	for _, iss := range res.Issues {
		keys = append(keys, iss.Key)
	}

	changelogs := make([][]jira.ChangelogHistory, len(keys))
	errs := make([]error, len(keys))

	sem := make(chan struct{}, sampleWorkers)
	var wg sync.WaitGroup
	for i, key := range keys {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			changelogs[i], errs[i] = api.ProxyChangelog(mc.client, key)
		}()
	}
	wg.Wait()

	// Other issues only add hints, so one failed fetch (a 429 under load, say) shouldn't
	// sink the search; the issue's own history is required.
	if errs[0] != nil {
		return 0, fmt.Errorf("fetching changelog of %s: %w", keys[0], errs[0])
	}
	sampled := 0
	for i, err := range errs {
		if err != nil {
			continue
		}
		g.addChangelog(changelogs[i])
		if i > 0 {
			sampled++
		}
	}

	return sampled, nil
}

// walk takes the hops one after the other. Each hop looks the transitions up again, since which ones
// are available depends on the status the issue is in by then.
func (mc *moveCmd) walk(hops []hop, installation string) error {
	key := mc.params.key

	for i, h := range hops {
		fail := func(err error) error {
			applied := ""
			if i > 0 {
				applied = fmt.Sprintf("; the %d earlier hop(s) stay applied", i)
			}
			return fmt.Errorf("hop %d of %d (%q -> %q): %w; %s is in status %q%s",
				i+1, len(hops), h.fromName, h.toName, err, key, h.fromName, applied)
		}

		available, err := withSpinner("Fetching available transitions...", func() ([]*jira.Transition, error) {
			ts, err := api.ProxyTransitionsWithFields(mc.client, key)
			return jira.AvailableTransitions(ts, installation), err
		})
		if err != nil {
			return fail(err)
		}

		tr, err := pickTransition(available, h.toID, h.toName)
		if err != nil {
			return fail(err)
		}
		if req := requiredFields(tr); len(req) > 0 {
			return fail(fmt.Errorf("transition %q requires fields (%s) that --path cannot provide", tr.Name, strings.Join(req, ", ")))
		}

		_, err = withSpinner(fmt.Sprintf("Transitioning issue to %q...", h.toName), func() (struct{}, error) {
			_, err := mc.client.Transition(key, &jira.TransitionRequest{
				Transition: &jira.TransitionRequestData{ID: tr.ID.String(), Name: tr.Name},
			})
			return struct{}{}, err
		})
		if err != nil {
			return fail(err)
		}

		cmdutil.Success("Issue %s moved to %q (via %q)", key, h.toName, tr.Name)
	}

	return nil
}

func printPath(hops []hop, available []*jira.Transition, project, issueType string, sampled int) {
	fmt.Printf("Path from %q to %q (learnt from the history of %d %s %s issues):\n",
		hops[0].fromName, hops[len(hops)-1].toName, sampled, project, issueType)

	for i, h := range hops {
		line := fmt.Sprintf("  %d. %s -> %s", i+1, h.fromName, h.toName)

		// Only the transitions out of the current status are known by name.
		if i == 0 {
			if tr, err := pickTransition(available, h.toID, h.toName); err == nil {
				via := fmt.Sprintf("via %q", tr.Name)
				if req := requiredFields(tr); len(req) > 0 {
					via += ", requires " + strings.Join(req, ", ")
				}
				line += "   (" + via + ")"
			}
		}
		fmt.Println(line)
	}

	fmt.Println("Run again with --yes to apply it.")
}

// resolveStatuses finds the ids of the target status by name, and which statuses are done. Names are shared
// by several statuses, eg. every team-managed project has its own "To Do", so all of them are targets.
func resolveStatuses(statuses []jira.Status, targetName string) (map[string]bool, map[string]bool) {
	targets := map[string]bool{}
	done := map[string]bool{}

	for _, s := range statuses {
		if strings.EqualFold(s.Name, targetName) {
			targets[s.ID] = true
		}
		if s.StatusCategory.Key == doneCategory {
			done[s.ID] = true
		}
	}

	return targets, done
}

func escapeJQL(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}

func withSpinner[T any](msg string, fn func() (T, error)) (T, error) {
	s := cmdutil.Info(msg)
	defer s.Stop()

	return fn()
}
