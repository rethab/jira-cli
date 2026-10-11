# CLAUDE.md

This repository is a fork of [ankitpokhrel/jira-cli](https://github.com/ankitpokhrel/jira-cli). It carries fixes and
features that upstream hasn't merged, and publishes its own releases.

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
