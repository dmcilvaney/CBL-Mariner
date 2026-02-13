---
name: "PRD Review: Agent Instructions"
description: "Reviews PR changes against the Agent Instructions PRD and flags anything that contradicts the PRD's design decisions, principles, or requirements."

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

# PRD Review: Agent Instructions

Review the pull request against the **Agent Instructions** PRD.

**Pull Request:** Identify the PR to review:

- If this is a `pull_request` trigger: review PR #${{ github.event.pull_request.number }}
- If this is a `workflow_dispatch` trigger: check the trigger context for a PR URL, then use the GitHub tools to resolve it

**PRD file:** `docs/prds/agent-instructions.prd.md`

Read this PRD file first, then follow the review process from the shared instructions below.

Pay special attention to these PRD-specific concerns:

- **Content placement** — Is content in the right mechanism? (domain knowledge → `.instructions.md`, guardrails → `AGENTS.md`, task workflows → skills)
- **Design principles** — "passive context first", "cross-platform by default", "minimize context rot"
- **Two-tier prompt model** — Tier 1 (skill-backed) vs Tier 2 (orchestrator) prompts
- **Skill design** — Skills for task workflows only, not domain knowledge; descriptions must include trigger phrases
- **Scope boundaries** — v1 in-scope vs out-of-scope items
