# CLAUDE.md

This repository is a fork of [ankitpokhrel/jira-cli](https://github.com/ankitpokhrel/jira-cli). It carries fixes and
features that upstream hasn't merged, and publishes its own releases.

## Pull requests

Open PRs against this fork, never against upstream. `gh` can default to the parent repository of a fork, so always
pass `--repo rethab/jira-cli --base main` to `gh pr create`.

## Releases

Every merge to `main` publishes a `snapshot-*` prerelease and a `jira-cli-snapshot` cask. A real release is cut by
pushing a `v*` tag, which runs `.github/workflows/release.yml`: GoReleaser builds the archives, creates the GitHub
release, and pushes the `jira-cli` cask to `rethab/homebrew-jira-cli`; the workflow then attests provenance. The same
tag also triggers `.github/workflows/docker.yml`, which publishes `ghcr.io/rethab/jira-cli:X.Y.Z`.

1. Make sure every workflow on the `main` commit you want to release is green (`gh run list --repo rethab/jira-cli
   --branch main --commit <sha>`).
2. Tag that commit and push the tag:
   ```sh
   git tag -a vX.Y.Z <sha> -m vX.Y.Z
   git push origin vX.Y.Z
   ```
3. Watch the Release and Docker workflows (`gh run watch <id> --repo rethab/jira-cli --exit-status`), then check the
   release page and the tap commit.

The fork versions independently of upstream, starting at v3.0.0, so upstream's v1.x tags never collide with ours. Bump
the minor version for new features and the patch version for fixes only.

## README: "Additional features"

`README.md` has an "About this fork" section with an "Additional features" list. It is how users tell this fork apart
from upstream, so keep it accurate.

When a change adds a user-facing feature that upstream doesn't have (a new command, flag, or visible behavior), the
same PR must:

- Add a bullet to "Additional features": a bold short name, the command or flag, one sentence on what it does, and a
  link to the upstream issue or PR (`([upstream #N](https://github.com/ankitpokhrel/jira-cli/issues/N))`) if one exists.
- Document the usage in the matching section under "Commands", like any other feature.

Bug fixes, dependency bumps, and CI changes don't get a bullet. If upstream ships a listed feature and we pick it up,
remove the bullet.
