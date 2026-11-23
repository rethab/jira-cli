# Prompt template: reviewer (Opus)

Fill in `{{PR}}`, `{{BRANCH}}`, `{{LEAK_CHECK}}` (absolute path of `scripts/leak-check.sh`) and `{{FORBID}}`
(`--forbid <absolute path of the user's input file>`, or empty if there is none), then pass everything below the
line as the agent prompt.

---

You are reviewing and fixing PR #{{PR}} (branch `{{BRANCH}}`) of `rethab/jira-cli`, a Go CLI for Jira. You didn't
write it, which is why you're the one reviewing it. You are in a git worktree of the repository. Read `CLAUDE.md`
first. Ignore `GAPS.md` and any file the user wrote the feature request in. Never read, stage, commit or copy one.

The user works against a private Jira instance, and this machine is also logged in to a second GitHub account. The
PR must contain none of the user's instance details: server host or site name, login or mail domain, project or
issue keys, board or team names, account IDs, instance-specific custom field IDs, real issue content, or any GitHub
account other than `rethab`. Treat any such detail you find in the diff, a fixture, a commit message or the PR body
as a review finding. Fix it with the repo's placeholders (`example.atlassian.net`, `TEST-1`, made-up IDs). Don't make
requests to a real Jira instance. Stage explicit paths only, never `git add -A` or `git add .`. Only use `gh` as
`rethab`.

1. Check out the PR branch: `git fetch origin {{BRANCH}} && git switch {{BRANCH}}` (or
   `gh pr checkout {{PR}} --repo rethab/jira-cli`). If `gh` fails with auth errors or a 404, run
   `gh auth switch --user rethab`.
2. Invoke the `simplify` skill on the changes this branch makes against `origin/main`. The implementer is new to the
   codebase, so look hardest for code that duplicates an existing helper (HTTP calls, pagination, output and table
   formatting, flag parsing) instead of reusing it. Keep only changes that leave the behavior as it is, and commit
   them locally ("Simplify ..."), without pushing yet.
3. Run the code-review skill on the branch, simplifications included, with fixes applied to the working tree: invoke
   the `code-review` skill with args `--fix {{BRANCH}}`. Doing this after step 2 means it also catches anything the
   simplification broke. Don't pass `max` or `ultra`.
4. Look at what both skills changed. Keep changes that are real improvements. Revert anything that changes the
   feature's intended behavior, isn't covered by a test where it should be, or adds narrating comments. Add or adjust
   tests for any behavior a fix changes.
5. Check that `README.md` still matches the code ("Additional features" bullet and "Commands" docs), as `CLAUDE.md`
   requires.
6. Run `make ci`. It must pass.
7. If anything changed, commit it without setting an author ("Address review findings" or something more specific).
   Run `{{LEAK_CHECK}} branch {{FORBID}}` and fix every hit before you push. If the fixes change what the PR does,
   run `{{LEAK_CHECK}} text <file> {{FORBID}}` on the new description, then update it with
   `gh pr edit {{PR}} --repo rethab/jira-cli --body-file <file>`.
8. Run `{{LEAK_CHECK}} pr {{PR}} {{FORBID}}`. If something already on GitHub fails it (an earlier commit, the
   PR body), fix what you can without rewriting pushed history, and report the rest.
9. `gh pr checks {{PR}} --repo rethab/jira-cli --watch` until green. Fix failures as they come, at most three
   rounds, then stop and report.

End with exactly:

```
HEAD: <sha of the PR head after your push, or the unchanged head>
STATUS: green | red
```

followed by a short list of the findings you fixed and any you deliberately left alone, with the reason, and the
last `leak-check.sh pr` result.
