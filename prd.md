---
title: "Evaluate azldev developer experience with agent-based scenarios"
status: Draft
date: 2026-07-20
owner: Daniel McIlvaney
tags: [azldev, developer-experience, ai]
---

# PRD-0001: Evaluate azldev developer experience with agent-based scenarios

## Summary

Azure Linux will create a repeatable, agent-based evaluation suite for
[`azldev`](https://github.com/microsoft/azure-linux-dev-tools). An autonomous
coding agent will attempt realistic development tasks using the same CLI help
and human-facing documentation available to a contributor. Deterministic
project checks will establish correctness, while transcripts and an independent
evaluation will identify workflow, error-message, and documentation friction.
The initial suite is report-only and complements rather than replaces feedback
from people.

## Problem & Motivation

`azldev` automated tests verify that capabilities work, but not whether a
contributor can discover and complete the intended workflow. We lack repeatable
evidence for questions such as:

- Can a developer find the correct workflow from the CLI and documentation?
- Do errors explain how to recover?
- Does a release add unnecessary retries or hidden expert steps?
- Which documentation gaps repeatedly block otherwise valid work?

Coding agents, while not truly representative of human developers, can attempt a fixed task repeatedly in an isolated environment.
Deterministic checks answer whether the task was completed, and the visible
execution trace shows where the workflow caused friction.

Grading agents convert the workflow into a structured report with numerical values assigned to each axis. These values can be used to track trends over time.

## Goals

- Provide realistic, goal-oriented scenarios for representative `azldev`
  workflows.
- Produce an authoritative pass, partial, fail, error, or unsafe result from
  deterministic project checks.
- Preserve enough evidence to explain every result.
- Identify actionable CLI, error-message, and documentation improvements.
- Support repeatable evaluation across `azldev`, Azure Linux, model, and runtime
  revisions.
- Establish a useful report-only proof of value before considering broader
  automation or release policy.

## Non-Goals

- Replace human usability research or describe synthetic results as human CSAT.
- Gate releases in the initial versions.
- Grade private model reasoning; only visible messages, tool activity, outputs,
  and final project state are evaluated.

## Requirements

| Priority | Requirement |
|----------|-------------|
| Must | Each scenario defines a realistic starting state, a natural-language goal, a timeout, and deterministic acceptance checks. It must not prescribe the command sequence or expose the expected patch. |
| Must | Runs use a disposable project workspace and never modify the input checkout. |
| Must | The baseline profile exposes human-facing documentation, source, configuration, CLI help, and normal development tools, but removes Azure Linux-specific agent instructions, skills, prompts, MCP configuration, hidden assertions, and prior results. |
| Must | Scenario setup verifies that the intended initial failure or unmet condition exists before the actor runs. An invalid starting state is a harness error, not a successful task. |
| Must | Deterministic checks are authoritative for correctness. An evaluator cannot convert failed render, lock, build, or scenario checks into a pass. |
| Must | The actor runs autonomously without human intervention and is stopped after the scenario timeout. |
| Must | Every run records the scenario and environment revisions, model identifiers, elapsed time, process status, deterministic results, final diff, and visible actor transcript. |
| Must | A separate evaluator reviews the task, transcript, final diff, and deterministic evidence. A different model family should be used where practical. |
| Must | Results include a machine-readable grade and a concise human-readable summary with supporting evidence. |
| Must | Harness, authentication, environment, timeout, and upstream failures remain distinguishable from product-task failures where evidence permits. |
| Must | v0.1 provides one locally runnable overlay diagnosis-and-repair scenario and remains report-only. |
| Should | v1 adds component import/build and image diagnosis/repair scenarios. |
| Should | v1 supports arbitrary `azldev` and project revisions plus report-only CI artifact publication. |
| Should | Repeated runs preserve trial and runtime metadata so maintainers can separate product changes from model variance. |
| Could | Later profiles may compare the baseline with agent instructions, skills, MCP tools, SDK integration, or IDE execution. |

### Result layers

Each run produces two complementary result layers:

- **Task result** (deterministic): pass, partial, fail, error, or unsafe — derived from project checks (render, lock, build, scenario invariants). Authoritative for correctness.
- **Axis scores** (evaluative): numerical ratings on dimensions such as usability, discoverability, and error-recovery quality — derived from an agent reviewing the transcript and evidence.

## User Scenarios

### Maintainer checks a candidate change

A maintainer runs the same scenario against two revisions, inspects the task
grade, transcript, and documentation findings. The result highlights meaningful
workflow changes.

### Maintainer investigates a failed run

A scenario fails a render or lock check. The maintainer distinguishes an
incorrect agent edit from a harness or upstream failure via the final diff and
transcript evidence.

### Team triages recurring usability friction

A scenario repeatedly scores low on a specific axis. The team reviews the transcript and documentation findings to identify a fix.

## Success Metrics

v0.1 succeeds when:

1. A maintainer can run the overlay scenario locally with one documented
   command.
2. The run produces an inspectable transcript, final diff, structured grade,
   and concise summary.
3. `azldev` maintainers judge the output useful for finding at least one
   product or documentation improvement during proof-of-value trials.

## Dependencies & Risks

- **Agent behavior is model-dependent.** Every run must record the model and
  runtime. Comparisons require a controlled environment and, eventually,
  multiple trials.
- **Synthetic behavior differs from human behavior.** Findings must be labeled
  as synthetic signals and periodically checked against feedback from
  contributors.
- **Upstream services introduce noise.** Fedora, source archives, mirrors, and
  package repositories may fail independently of `azldev`; result evidence must
  preserve that distinction.
- **Public scenarios can be overfit.** The suite should grow to include variants
  and, where justified, held-out fixtures.
- **Real builds can be expensive.** Initial fixtures should use small
  components and avoid full image builds until the scenarios are stable.
- **Agent execution handles untrusted content.** Runs require an isolated
  environment without unnecessary host paths, credentials, or control sockets.

## Open Questions

1. Which small component provides the most representative overlay-repair
   fixture?
2. Which component and image should anchor the next scenario families?
3. What evidence is sufficient to classify an upstream failure automatically?
4. Where should the harness implementation live?

## References

- [`azldev` repository](https://github.com/microsoft/azure-linux-dev-tools)
