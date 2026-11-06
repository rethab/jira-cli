---
name: api-gap
description: Take a claim that the Jira REST API can do something this CLI can't, check it, and if it holds, ship it end to end. The steps are an Opus investigation that accepts or rejects the claim, a Sonnet TDD implementation with a PR, an Opus /code-review --fix pass, a squash merge, and a release. Use this whenever the user says something like "the API supports X but jira-cli doesn't", "there's no command for Y but the REST endpoint exists", "I had to curl the API to do Z", or asks to add a command or flag that wraps an existing Jira endpoint. Use it even when they don't say "API" outright, e.g. "why can't I do X with jira, Jira itself allows it".
---

# api-gap: from "the API can do it" to a release

You are the orchestrator. Agents do the heavy work; you pass state between them, enforce the gates, and own the
outward-facing steps at the end (merge, tag). The pipeline is deliberately split across fresh agents. The Opus
investigator judges the claim without being invested in an implementation, and the Opus reviewer reads code it
didn't write. Don't collapse stages into one agent, and don't do an agent's job yourself, even when it looks quick.

Repository facts every stage relies on (all from `CLAUDE.md`, which each agent should read too):

- Fork of `ankitpokhrel/jira-cli`. PRs go to `rethab/jira-cli`, always with `--repo rethab/jira-cli --base main`.
- `gh` errors or 404s usually mean the wrong account: `gh auth switch --user rethab`.
- Ignore `GAPS.md` in the repo root. It's the user's private scratch file: don't read it, act on it, stage it, or
  commit it.

Templates for the three agent prompts are in `references/`. Read the matching one right before spawning each agent,
fill in the placeholders, and pass the result as the prompt. They carry the detailed instructions, so the agent
doesn't need this file.

## Stage 1: Investigate (Opus)

Spawn a `general-purpose` agent with `model: "opus"` and the prompt from `references/investigate.md`. Give it the
user's claim verbatim, plus any endpoint, docs link, or example they gave. No worktree is needed because it only
reads.

It ends with `VERDICT: REJECT` or `VERDICT: ACCEPT` and the absolute path of a plan file.

- **REJECT**: relay its reasons to the user (claim false, CLI already does it, upstream has it, or it doesn't fit
  the design) and stop. A rejection is a successful outcome. Don't argue with it or re-spawn hoping for a different
  answer. If the user disagrees and adds new information, run Stage 1 again with that information.
- **ACCEPT**: read the plan file yourself, check that it names the API endpoints, the files to touch, the tests to
  write first, and the README bullet, then give the user a 3–5 line summary of what will be built and continue.

## Stage 2: Implement test-first and open the PR (Sonnet)

Spawn a `general-purpose` agent with `model: "sonnet"`, `isolation: "worktree"`, and the prompt from
`references/implement.md`, passing the plan file's absolute path. The worktree keeps the user's checkout (and
`GAPS.md`) untouched.

It returns a PR number and branch name once the PR's checks are green. If it reports that checks are still red after
its fix attempts, or that the plan doesn't work, stop and tell the user what it found. Don't push past a red PR.

Check it yourself before moving on: `gh pr checks <N> --repo rethab/jira-cli` must show every check passing.

## Stage 3: Code review with fixes (Opus)

Spawn a `general-purpose` agent with `model: "opus"`, `isolation: "worktree"`, and the prompt from
`references/review.md`, passing the PR number and branch. It checks out the PR branch, runs `/code-review --fix`,
pushes the fixes, and waits for green again.

When it returns, check again: the PR head must be the commit it reports pushing (or unchanged if there was nothing to
fix), and every check on it must pass.

## Stage 4: Squash merge

Run this yourself, from the main checkout, without switching its branch:

```sh
gh pr view <N> --repo rethab/jira-cli --json title,body,mergeable,headRefOid
gh pr merge <N> --repo rethab/jira-cli --squash --delete-branch \
  --subject "<title> (#<N>)" --body "<PR body>"
```

Passing title and body explicitly keeps the history format of earlier merges (subject `Title (#N)`, body = PR
description). Write the body to a file and use `--body-file` if it contains backticks or quotes. If the PR isn't
mergeable (conflict with a newer `main`), stop and tell the user. Don't rebase it yourself.

## Stage 5: Release

Follow the "Releases" section of `CLAUDE.md`. In short:

1. `git fetch origin main --tags`, then take the merge commit's sha (`git rev-parse origin/main`, and check that its
   subject ends with `(#<N>)`).
2. Wait for every workflow on that commit to finish green:
   `gh run list --repo rethab/jira-cli --branch main --commit <sha>`. Watch each one with
   `gh run watch <id> --repo rethab/jira-cli --exit-status`. A red run on `main` blocks the release, so stop and
   report it.
3. Pick the version from the newest `v*` tag (`git tag --sort=-v:refname | head -1`). Bump the minor version for a new
   command, flag, or behavior and reset the patch (v3.8.0 → v3.9.0). Bump the patch only if the change was purely a
   fix.
4. `git tag -a vX.Y.Z <sha> -m vX.Y.Z && git push origin vX.Y.Z`
5. Watch the Release and Docker runs the tag triggers to completion, then check that the GitHub release exists
   (`gh release view vX.Y.Z --repo rethab/jira-cli`).

## Report

Finish with a short summary: the claim, the verdict, the PR link, what the review changed, the merge commit, and the
release version and its URL. If you stopped early, say at which stage and why, and what state things were left in
(open PR, pushed branch, and so on), so the user can pick up from there.
