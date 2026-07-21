# azldev tooling-evaluation harness (spike v0.1)

Evaluates the **`azldev` tooling** — its CLI, error messages, and human docs — by
having a baseline coding agent (no Azure Linux AI helpers) diagnose and repair a
deliberately broken component, gated by deterministic `azldev` checks and graded
by a separate tool-focused evaluator. Report-only. Never modifies your checkout.

## Run it

```bash
python3 eval/run_eval.py
```

That runs the `cpio-overlay` scenario end to end and writes a run under
`eval/runs/<timestamp>__cpio-overlay/`:

- `run.json` — canonical record (env, models, timing, deterministic result, evaluation)
- `summary.md` — human-readable at-a-glance
- `transcript.txt` — the actor agent's visible transcript (`tail -f` it live)
- `fix.diff` — the agent's change vs the seeded baseline
- `checks/{build,render,lock}.log` — deterministic gate output
- `evaluation.json` — raw evaluator output

### Useful flags

| Flag | Purpose |
| ---- | ------- |
| `--seed-only` | Seed + verify the precondition, then stop (harness self-check; no agent) |
| `--skip-evaluator` | Run the deterministic phases only |
| `--keep-workspace` | Keep the disposable workspace for inspection |
| `--actor-model`, `--evaluator-model` | Override the models (`copilot --model` IDs) |
| `--input-repo` | Evaluate a different checkout (default: this repo) |

## What a run does

1. Clone the checkout into a disposable workspace (never touches the input).
2. Strip Azure Linux AI scaffolding (instructions, skills, prompts, agents, MCP)
   so the agent has only what a human contributor does.
3. Seed the break (`scenarios/cpio-overlay/break.patch`) and flatten git history
   to a single baseline commit so the fix can't be recovered by reverting.
4. Re-render + re-lock so the *only* defect is the build; verify the precondition.
5. Run the actor agent (goal in `scenario.toml`), timeboxed.
6. Run the deterministic gates (`build`, `render --check-only`,
   `update --check-only`) → `pass` / `partial` / `fail`.
7. Run the evaluator (`evaluator.md` rubric) → axis scores + actionable findings.

## Deterministic result

| Result | Meaning |
| ------ | ------- |
| `pass` | build ok + specs re-rendered + locks updated (PR-ready) |
| `partial` | builds, but render/lock not reconciled |
| `fail` | still doesn't build |
| `error` | bad seed / timeout / harness failure |

The evaluator never overrides the deterministic result; it adds axis scores,
findings, and an `unsafe` flag.

## Add a scenario

Create `scenarios/<name>/` with `scenario.toml`, `break.patch`, and
`evaluator.md` (copy `cpio-overlay/`), then `--scenario eval/scenarios/<name>`.

> Spike scope: local + attended. Isolation (container), multi-trial, and
> synthetic test environments are deferred. See the design ledger.
