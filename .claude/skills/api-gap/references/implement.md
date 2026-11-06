# Prompt template: implementer (Sonnet)

Fill in `{{PLAN_PATH}}`, then pass everything below the line as the agent prompt.

---

You are implementing a planned feature in `jira-cli`, a Go CLI for Jira, and opening a PR for it. You are in a
git worktree of the repository. Read `CLAUDE.md` first: it covers where PRs go and the README rules. Ignore
`GAPS.md` if you see it.

The plan is at `{{PLAN_PATH}}`. Read it fully before you start. It was written after checking the API and the
surrounding code, so follow its design. If it turns out to be wrong in a way that matters (an endpoint behaves
differently, a helper it names doesn't exist), adapt in the plan's spirit and mention the deviation in your final
report. If it can't work at all, stop and report that instead of improvising a different feature.

## Work test-first

1. Branch off up-to-date `main`: `git fetch origin main && git switch -c <short-descriptive-name> origin/main`.
2. For each piece of behavior, write the test first, run it and watch it fail for the right reason, then write the
   minimum code to make it pass. Start at the `pkg/jira` client layer (`httptest` servers asserting path, method and
   JSON body, as in `pkg/jira/edit_test.go`), then the command layer. Seeing each test fail before the code exists is
   how you know it tests something.
3. Match the surrounding code: naming, flag definitions, output modes, error messages. Comments explain *why*, never
   *what*. Don't write comments that restate the next line.
4. Update `README.md`: the "Additional features" bullet and the usage docs under "Commands", as the plan and
   `CLAUDE.md` describe.
5. Run `make ci` (lint + race tests). Both must pass locally before you push.

## Commit and open the PR

- Commit without setting an author (a global one is configured). Use a short imperative subject, like the recent
  history (`git log --oneline -10`).
- `git push -u origin <branch>`
- `gh pr create --repo rethab/jira-cli --base main --title "<subject>" --body-file <file>`.
  The body explains what changed and **why**, in the style of earlier PRs (`gh pr view 71 --repo rethab/jira-cli`).
  Start with the problem the user hit, then describe the change at a high level. No "Summary" or "Test Plan"
  headings, and no file-by-file list.
- If `gh` fails with auth errors or a 404: `gh auth switch --user rethab` and retry.

## Get it green

`gh pr checks <N> --repo rethab/jira-cli --watch`. If a check fails, read its log
(`gh run view <run-id> --repo rethab/jira-cli --log-failed`), reproduce locally, fix, push, and watch again. After
three failed rounds, stop and report what is failing and why. Don't disable tests or lint rules to get past it.

## Report

End with exactly:

```
PR: <number>
BRANCH: <branch>
STATUS: green | red
```

followed by a few lines on any deviations from the plan.
