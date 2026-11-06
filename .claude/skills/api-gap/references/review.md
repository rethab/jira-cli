# Prompt template: reviewer (Opus)

Fill in `{{PR}}` and `{{BRANCH}}`, then pass everything below the line as the agent prompt.

---

You are reviewing and fixing PR #{{PR}} (branch `{{BRANCH}}`) of `rethab/jira-cli`, a Go CLI for Jira. You didn't
write it, which is why you're the one reviewing it. You are in a git worktree of the repository. Read `CLAUDE.md`
first. Ignore `GAPS.md` if you see it.

1. Check out the PR branch: `git fetch origin {{BRANCH}} && git switch {{BRANCH}}` (or
   `gh pr checkout {{PR}} --repo rethab/jira-cli`). If `gh` fails with auth errors or a 404, run
   `gh auth switch --user rethab`.
2. Run the code-review skill on the PR with fixes applied to the working tree: invoke the `code-review` skill with
   args `--fix {{PR}}`. Don't pass `max` or `ultra`.
3. Look at what it changed. Keep fixes that are real improvements. Revert anything that changes the feature's
   intended behavior, isn't covered by a test where it should be, or adds narrating comments. Add or adjust tests
   for any behavior a fix changes.
4. Check that `README.md` still matches the code ("Additional features" bullet and "Commands" docs), as `CLAUDE.md`
   requires.
5. Run `make ci`. It must pass.
6. If anything changed, commit it without setting an author ("Address review findings" or something more specific),
   then push. If the fixes change what the PR does, update its description with
   `gh pr edit {{PR}} --repo rethab/jira-cli --body-file <file>`.
7. `gh pr checks {{PR}} --repo rethab/jira-cli --watch` until green. Fix failures as they come, at most three
   rounds, then stop and report.

End with exactly:

```
HEAD: <sha of the PR head after your push, or the unchanged head>
STATUS: green | red
```

followed by a short list of the findings you fixed and any you deliberately left alone, with the reason.
