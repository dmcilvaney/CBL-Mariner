---
name: "PRD Review: Distro Config Layout"
description: "Reviews PR changes against the Distro Config Layout PRD and flags anything that contradicts the PRD's design decisions, principles, or requirements."

on:
  pull_request:
    types: [opened, synchronize, reopened]
  workflow_dispatch:

permissions:
  contents: read
  issues: read
  pull-requests: read

imports:
  - shared/prd-review.md
---

# PRD Review: Distro Config Layout

Review the pull request against the **Distro Config Layout** PRD.

**Pull Request:** Identify the PR to review:

- If this is a `pull_request` trigger: review PR #${{ github.event.pull_request.number }}
- If this is a `workflow_dispatch` trigger: check the trigger context for a PR URL, then use the GitHub tools to resolve it

**PRD file:** `docs/prds/distro-config-layout.prd.md`

Read this PRD file first, then follow the review process from the shared instructions below.

Pay special attention to these PRD-specific concerns:

- **Directory structure** — Are distro configs in `distro/`, not scattered elsewhere?
- **File naming** — Do new distro definitions follow `<name>.distro.toml` convention?
- **Include hierarchy** — Is `distro.toml` updated when new distro definitions are added?
- **Mock config conventions** — Do mock configs follow the `<distro>-<version>-<arch>.cfg` pattern?
- **Build default placement** — Are build defaults in distro definitions, not project or component files?
