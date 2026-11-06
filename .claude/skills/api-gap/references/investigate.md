# Prompt template: investigator (Opus)

Fill in `{{CLAIM}}` and `{{CONTEXT}}`, then pass everything below the line as the agent prompt.

---

You are judging whether a proposed feature should be added to `jira-cli`, a Go CLI for Jira. The repository is in the
current directory. It is a fork of `ankitpokhrel/jira-cli` (read `CLAUDE.md` first). Ignore `GAPS.md`.

The user says:

> {{CLAIM}}

Extra context they gave: {{CONTEXT}}

Your job is to decide whether to build it, and if so how. You don't write code. Be skeptical: users often remember the
API as more capable than it is, miss a CLI flag that already does the job, or ask for something that fits awkwardly
into the CLI. Rejecting a bad idea is as useful as planning a good one, so don't lean toward ACCEPT.

## 1. Is the claim true of the API?

Find out how the endpoint actually behaves. Don't rely on memory.

- Read the official Atlassian docs: Jira Cloud REST v3 and v2 (`developer.atlassian.com/cloud/jira/platform/rest/`),
  Jira Software/Agile (`/rest/agile/1.0`), and the Jira Server/Data Center REST docs. Use Context7 for library docs
  where it helps. Pin down the method, path, request body, response shape, required permissions, and pagination.
- The CLI supports both Cloud (`installation: Cloud`) and Server/DC (`installation: Local`). See `pkg/jira/client.go`
  for the v1/v2/v3 base URLs and grep `InstallationType` for where behavior already differs. Find out whether the
  feature exists on both, and where it differs (ADF vs. wiki markup, accountId vs. username, endpoint availability).
  A Cloud-only feature can still be fine if the CLI already has a pattern for that, but say so explicitly.
- If a Jira instance is configured for this CLI and reachable, you may make read-only GET requests to confirm response
  shapes. Never create, edit, or delete anything on a real instance.

## 2. Is it really missing from the CLI?

- Search `internal/cmd/**` and `pkg/jira/**` for the endpoint path and related terms, and check `--help` of the
  nearest commands (`go run ./cmd/jira <cmd> --help`). Something that's already doable (another flag, `--raw`,
  `--custom`, a JQL filter) doesn't need building. Point the user at it.
- Check upstream: `gh issue list` / `gh pr list --repo ankitpokhrel/jira-cli --search "<terms>" --state all`.
  If upstream has already shipped it, the fork should pick it up rather than reimplement it, so reject with that
  pointer. If an upstream issue or PR asks for it, record its number for the README `[#N]` link.

## 3. Does it fit the CLI's design?

Read the two or three commands closest to what's proposed, both the `internal/cmd/...` layer and its `pkg/jira`
client method with tests. Work out:

- Where it would live: new subcommand, new flag on an existing command, or a behavior change. Follow the existing
  command tree and naming (`jira issue <noun> <verb>`, short flags, `--plain`/`--raw`/`--no-input` conventions).
- How similar commands handle output (table, plain, raw JSON), prompts and `--no-input`, errors, and paging.
- Which client helpers and request/response types it would reuse.

Reject when the feature would need a new paradigm the CLI doesn't have, would duplicate a command under another
name, is so niche or admin-only that it's better as a `curl` call, or can't be made to behave sensibly on the
installation types the CLI supports. Say which of these applies.

## 4. Output

If rejecting, end your reply with:

```
VERDICT: REJECT
<2–6 lines: which check failed and the evidence (doc link, existing flag, upstream PR, design conflict)>
```

If accepting, write the plan to a markdown file in your scratchpad directory (or under `$TMPDIR` if you have none)
and end your reply with:

```
VERDICT: ACCEPT
PLAN: <absolute path>
```

The plan is all the implementer gets. It is a capable engineer who hasn't seen this conversation, so include:

- **Goal**: the user-facing behavior in one paragraph, including the exact command/flag syntax and an example.
- **API**: method, path, body and response for each endpoint, with doc links, and Cloud vs. Server differences.
- **Design**: where it goes and which existing command it mirrors (file paths), with the reasoning, so the
  implementer can make sensible calls on details you didn't anticipate.
- **Changes**: files to add or modify in `pkg/jira` and `internal/cmd`, the types and functions to add, and the
  helpers to reuse.
- **Tests first**: the failing tests to write before the code, in the repo's style (`httptest` server asserting
  path, method and JSON body in `pkg/jira/*_test.go`, fixtures in `pkg/jira/testdata`, command tests next to the
  command).
- **Docs**: the "Additional features" bullet for `README.md` (group, wording, upstream `[#N]` link if any), and
  which "Commands" section gets the usage docs.
- **Out of scope**: things deliberately left out, so the implementer doesn't add them.
