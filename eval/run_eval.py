#!/usr/bin/env python3
"""azldev tooling-evaluation harness (spike v0.1).

Pipeline: clone checkout -> strip AZL agent scaffolding -> seed the break
(flatten git history so it can't be reverted) -> re-render/-lock so the *only*
defect is the build -> check precondition -> run actor -> run deterministic
gates -> run evaluator -> write a run under runs/. Report-only; never touches
the input checkout. See README.md.
"""

from __future__ import annotations

import argparse
import io
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import tomllib
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

EVAL_DIR = Path(__file__).resolve().parent
REPO_ROOT = EVAL_DIR.parent
WORK_DIR = REPO_ROOT / "base" / "build" / "work" / "scratch" / "eval-work"
RUNS_DIR = EVAL_DIR / "runs"

# AZL AI-specific helpers removed so the actor sees only what a human contributor
# has: CLI help, source, config, human docs.
STRIP = [
    ".github/copilot-instructions.md",
    ".github/instructions",
    ".github/skills",
    ".github/prompts",
    ".github/agents",
    ".github/plugin",
    "AGENTS.md",
    "distro/AGENTS.md",
    "base/comps/AGENTS.md",
    ".vscode/mcp.json",
    "scripts/mcps",
    "prd.md",
    "docs/prds",
    "eval",  # the harness itself — contains break.patch (the answer)
]
GIT_ENV = {
    "GIT_AUTHOR_NAME": "azldev-eval",
    "GIT_AUTHOR_EMAIL": "eval@localhost",
    "GIT_COMMITTER_NAME": "azldev-eval",
    "GIT_COMMITTER_EMAIL": "eval@localhost",
}
TIMEOUT_RC = 124

type JSON = dict[str, Any]


class HarnessError(RuntimeError):
    """Environment/harness failure — distinct from an actor task failure."""


def run(
    cmd: list[str],
    *,
    cwd: Path,
    log: Path | None = None,
    env: dict[str, str] | None = None,
    timeout: float | None = None,
) -> tuple[int, str]:
    """Run cmd, combined output. `log` streams live to that file (watchable via tail -f).

    Timeout -> rc 124. Never raises on exit code.
    """
    full_env = {**os.environ, **(env or {})}
    if log is not None:
        with log.open("w", encoding="utf-8") as f:
            try:
                rc = subprocess.run(
                    cmd,
                    cwd=str(cwd),
                    env=full_env,
                    stdout=f,
                    stderr=subprocess.STDOUT,
                    timeout=timeout,
                    check=False,
                ).returncode
            except subprocess.TimeoutExpired:
                f.write("\n[harness: timeout]\n")
                rc = TIMEOUT_RC
        return rc, log.read_text(encoding="utf-8")
    try:
        p = subprocess.run(
            cmd,
            cwd=str(cwd),
            env=full_env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=timeout,
            check=False,
        )
        return p.returncode, p.stdout
    except subprocess.TimeoutExpired as e:
        return TIMEOUT_RC, (e.output or "") + "\n[harness: timeout]\n"


def git(args: list[str], *, cwd: Path) -> str:
    rc, out = run(["git", *args], cwd=cwd, env=GIT_ENV)
    if rc != 0:
        raise HarnessError(f"git {' '.join(args)} failed:\n{out}")
    return out.strip()


def azldev(args: list[str], *, cwd: Path, log: Path | None = None) -> int:
    rc, _ = run(["azldev", "-C", str(cwd), *args], cwd=cwd, log=log)
    return rc


def seed(ws: Path, scenario: JSON, scenario_dir: Path) -> str:
    """Apply break, flatten to one baseline commit, reconcile spec+lock. Returns baseline sha."""
    comp = scenario["component"]
    rc, out = run(["git", "apply", str(scenario_dir / scenario["break_patch"])], cwd=ws, env=GIT_ENV)
    if rc != 0:
        raise HarnessError(f"break.patch did not apply:\n{out}")
    shutil.rmtree(ws / ".git")
    git(["init", "--quiet"], cwd=ws)
    git(["add", "-A"], cwd=ws)
    git(["commit", "--quiet", "-m", "baseline"], cwd=ws)
    azldev(["component", "update", "-p", comp, "-q"], cwd=ws)
    # ponytail: the first (cold-chroot) render returns 0 without writing, so loop
    # render -> commit -> re-check until the spec is render-clean (converges on the
    # warm pass). Bounded; raises if it never settles.
    for _ in range(4):
        azldev(["component", "render", "-p", comp, "-q"], cwd=ws)
        git(["add", "-A"], cwd=ws)
        git(["commit", "--quiet", "--amend", "--no-edit"], cwd=ws)
        if azldev(["component", "render", "-p", comp, "--check-only", "-q"], cwd=ws) == 0:
            break
    else:
        raise HarnessError("seed did not converge to a render-clean baseline")
    if azldev(["component", "update", "-p", comp, "--check-only", "-q"], cwd=ws) != 0:
        raise HarnessError("seed lock not clean after render")
    return git(["rev-parse", "HEAD"], cwd=ws)


def gates(ws: Path, comp: str, checks_dir: Path) -> dict[str, int]:
    """Run the three cpio-scoped deterministic checks (exit codes; 0 = ok/clean)."""
    return {
        "build": azldev(["component", "build", "-p", comp], cwd=ws, log=checks_dir / "build.log"),
        "render": azldev(["component", "render", "-p", comp, "--check-only"], cwd=ws, log=checks_dir / "render.log"),
        "lock": azldev(["component", "update", "-p", comp, "--check-only"], cwd=ws, log=checks_dir / "lock.log"),
    }


def task_result(g: dict[str, int]) -> str:
    if g["build"] != 0:
        return "fail"
    return "pass" if g["render"] == 0 and g["lock"] == 0 else "partial"


def clean_home(path: Path) -> Path:
    """Build a scratch HOME with only copilot auth, neutralising the operator's global config."""
    cfg = Path.home() / ".copilot" / "config.json"
    gh = Path.home() / ".config" / "gh"
    if not cfg.exists():
        raise HarnessError(f"copilot auth config not found at {cfg}")
    if path.exists():
        shutil.rmtree(path)
    (path / ".copilot").mkdir(parents=True)
    shutil.copy2(cfg, path / ".copilot" / "config.json")
    if gh.exists():  # copilot validates via the gh token; config.json alone 401s
        shutil.copytree(gh, path / ".config" / "gh")
    return path


def copilot_env(home: Path) -> dict[str, str]:
    env = {"HOME": str(home)}
    for var in ("COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"):
        if os.environ.get(var):
            env[var] = os.environ[var]
    return env


def run_actor(ws: Path, scenario: JSON, model: str, home: Path, transcript: Path) -> tuple[str, float]:
    cmd = [
        "copilot",
        "--model",
        model,
        "--output-format",
        "json",  # full turn-by-turn JSONL (tool calls, outputs, responses) for the evaluator
        "--allow-all-tools",
        "--allow-all-paths",
        "--allow-all-urls",
        "-p",
        scenario["goal"],
    ]
    print(f"[actor] {model}, timeout {scenario['timeout_s']}s — tail -f {transcript}")
    start = time.monotonic()
    rc, _ = run(cmd, cwd=ws, log=transcript, env=copilot_env(home), timeout=scenario["timeout_s"])
    elapsed = round(time.monotonic() - start, 1)
    status = "timeout" if rc == TIMEOUT_RC else ("completed" if rc == 0 else "crashed")
    print(f"[actor] {status} in {elapsed}s")
    return status, elapsed


def run_evaluator(scenario_dir: Path, model: str, evidence: str, home: Path, log: Path) -> JSON:
    rubric = (scenario_dir / "evaluator.md").read_text(encoding="utf-8")
    with tempfile.TemporaryDirectory() as tmp:  # cwd holds evidence.md; judge reads it (argv can't hold 100s of KB)
        (Path(tmp) / "evidence.md").write_text(evidence, encoding="utf-8")
        prompt = f"{rubric}\n\n---\nRead `./evidence.md` in your working directory for the run evidence, then grade.\n"
        cmd = ["copilot", "--model", model, "--allow-all-tools", "--allow-all-paths", "-p", prompt]
        print(f"[evaluator] {model}")
        _, out = run(cmd, cwd=Path(tmp), log=log, env=copilot_env(home), timeout=900)
    return extract_json(out)


def extract_json(text: str) -> JSON:
    m = re.search(r"```(?:json)?\s*(\{.*?\})\s*```", text, re.DOTALL)
    blob = m.group(1) if m else text[text.find("{") : text.rfind("}") + 1]
    try:
        return json.loads(blob)
    except (json.JSONDecodeError, ValueError) as e:
        return {"_parse_error": str(e), "_raw": text}


_SUBSTANTIVE_EVENTS = {"assistant.message", "tool.execution_complete"}


def distill_transcript(raw: str) -> str:
    """Reduce actor JSONL to reasoning + tool calls + tool results.

    Drops streaming deltas, session/MCP-status noise, and opaque reasoning blobs.
    Reasoning survives via `assistant.message.reasoningText` (it is not a separate
    kept event), so filtering on `ephemeral` is wrong — it would strip the thinking.
    """
    out: list[str] = []
    for ln in raw.splitlines():
        try:
            ev = json.loads(ln)
        except (json.JSONDecodeError, ValueError):
            continue
        if ev.get("type") not in _SUBSTANTIVE_EVENTS:
            continue
        d = ev.get("data") or {}
        if ev["type"] == "assistant.message":
            if d.get("reasoningText"):
                out.append(f"[reasoning]\n{d['reasoningText']}")
            for tr in d.get("toolRequests") or []:
                a = tr.get("arguments") or {}
                out.append(f"[tool:{tr.get('name')}] {a.get('command', a)}")
            if d.get("content"):
                out.append(f"[assistant]\n{d['content']}")
        else:  # tool.execution_complete
            content = (d.get("result") or {}).get("content", "")
            out.append(f"[result]\n{content}")
    return "\n\n".join(out)


def evidence_bundle(goal: str, transcript: str, diff: str, log: str, checks: dict[str, str]) -> str:
    checks_txt = "\n".join(f"### {k}\n```\n{v.strip()}\n```" for k, v in checks.items())
    return (
        f"## Goal\n{goal}\n\n## Deterministic checks\n{checks_txt}\n\n"
        f"## Final diff\n```diff\n{diff.strip()}\n```\n\n"
        f"## Run git log\n```\n{log.strip()}\n```\n\n## Transcript\n```\n{transcript.strip()}\n```\n"
    )


def write_summary(run_dir: Path, record: JSON) -> None:
    ev = record.get("evaluation", {})
    axes = "\n".join(
        f"- **{k}**: {v.get('score')}/5 — {v.get('rationale', '')}"
        for k, v in ev.get("axis_scores", {}).items()
        if isinstance(v, dict)
    )
    finds = "\n".join(
        f"- `{f.get('area')}` — {f.get('problem')} → {f.get('suggestion')}"
        for f in ev.get("findings", [])
        if isinstance(f, dict)
    )
    (run_dir / "summary.md").write_text(
        f"# Eval run {run_dir.name}\n\n"
        f"- **task_result**: `{record['deterministic'].get('task_result')}`\n"
        f"- **actor**: {record['timing'].get('status')} ({record['timing'].get('actor_elapsed_s')}s)\n"
        f"- **models**: actor=`{record['models']['actor']}`, evaluator=`{record['models']['evaluator']}`\n\n"
        f"## Acceptance\n```json\n{json.dumps(record['deterministic'].get('acceptance', {}), indent=2)}\n```\n\n"
        f"## Axis scores\n{axes or '_none_'}\n\n## Findings\n{finds or '_none_'}\n\n"
        f"## Summary\n{ev.get('summary', '_no evaluation_')}\n",
        encoding="utf-8",
    )


def orchestrate(args: argparse.Namespace) -> int:
    scenario_dir = Path(args.scenario).resolve()
    scenario = tomllib.loads((scenario_dir / "scenario.toml").read_text(encoding="utf-8"))
    scenario["goal"] = scenario["goal"].strip()
    comp = scenario["component"]
    input_repo = Path(args.input_repo).resolve()

    stamp = datetime.now(UTC).strftime("%Y%m%dT%H%M%SZ")
    tag = f"{stamp}__{scenario['id']}"
    run_dir, ws, home = RUNS_DIR / tag, WORK_DIR / tag, WORK_DIR / f"{tag}.home"
    checks_dir = run_dir / "checks"
    checks_dir.mkdir(parents=True, exist_ok=True)

    print(f"=== eval {tag} ===")
    record = {
        "scenario": {
            "id": scenario["id"],
            "component": comp,
            "goal": scenario["goal"],
            "timeout_s": scenario["timeout_s"],
        },
        "env": {
            "repo_sha": git(["rev-parse", "HEAD"], cwd=input_repo),
            "azldev_version": run(["azldev", "--version"], cwd=REPO_ROOT)[1].strip(),
            "host": "local",
        },
        "models": {"actor": args.actor_model, "evaluator": args.evaluator_model},
        "timing": {"started": stamp},
        "deterministic": {},
        "evaluation": {},
    }

    try:
        if ws.exists():
            shutil.rmtree(ws)
        ws.parent.mkdir(parents=True, exist_ok=True)
        print(f"[prepare] cloning -> {ws}")
        git(["clone", "--local", "--no-hardlinks", "--quiet", str(input_repo), str(ws)], cwd=input_repo)
        record["env"]["removed_scaffolding"] = strip(ws)
        record["env"]["baseline_sha"] = baseline = seed(ws, scenario, scenario_dir)
        print("[precondition] verifying build fails, render/lock clean")
        pre = gates(ws, comp, checks_dir)
        if pre["build"] == 0 or pre["render"] != 0 or pre["lock"] != 0:
            raise HarnessError(f"bad seed (build={pre['build']} render={pre['render']} lock={pre['lock']})")
    except HarnessError as e:
        print(f"[ERROR] {e}")
        record["deterministic"] = {"task_result": "error", "reason": str(e)}
        (run_dir / "run.json").write_text(json.dumps(record, indent=2), encoding="utf-8")
        return 2

    if args.seed_only:
        print(f"[seed-only] precondition OK; workspace at {ws}")
        return 0

    status, elapsed = run_actor(ws, scenario, args.actor_model, clean_home(home), run_dir / "transcript.txt")
    record["timing"].update(status=status, actor_elapsed_s=elapsed)

    print("[acceptance] running deterministic gates")
    g = gates(ws, comp, checks_dir)
    record["deterministic"] = {
        "acceptance": {
            "build": "pass" if g["build"] == 0 else "fail",
            "render_drift": g["render"] != 0,
            "lock_drift": g["lock"] != 0,
        },
        "task_result": task_result(g),
    }
    print(f"[acceptance] task_result = {record['deterministic']['task_result']}")

    diff = git(["diff", baseline], cwd=ws)
    log = git(["log", "--stat", f"{baseline}..HEAD"], cwd=ws) or "(no commits)"
    log += "\n\n# working-tree changes:\n" + (git(["status", "--porcelain"], cwd=ws) or "(none)")
    (run_dir / "fix.diff").write_text(diff, encoding="utf-8")
    (run_dir / "git-log.txt").write_text(log, encoding="utf-8")

    if not args.skip_evaluator:
        raw = (run_dir / "transcript.txt").read_text(encoding="utf-8")
        transcript = distill_transcript(raw)  # reasoning + tool calls + results; no delta/blob noise
        checks = {p.stem: p.read_text(encoding="utf-8") for p in sorted(checks_dir.glob("*.log"))}
        record["evaluation"] = run_evaluator(
            scenario_dir,
            args.evaluator_model,
            evidence_bundle(scenario["goal"], transcript, diff, log, checks),
            home,
            run_dir / "evaluator-raw.log",
        )
        (run_dir / "evaluation.json").write_text(json.dumps(record["evaluation"], indent=2), encoding="utf-8")

    (run_dir / "run.json").write_text(json.dumps(record, indent=2), encoding="utf-8")
    write_summary(run_dir, record)

    if not args.keep_workspace:
        shutil.rmtree(ws, ignore_errors=True)
        shutil.rmtree(home, ignore_errors=True)
    print(f"=== done: {run_dir} ===")
    return 0


def strip(ws: Path) -> list[str]:
    removed = []
    for rel in STRIP:
        t = ws / rel
        if t.is_dir():
            shutil.rmtree(t)
            removed.append(rel)
        elif t.exists():
            t.unlink()
            removed.append(rel)
    for env in ws.rglob(".env"):
        if ".git" not in env.parts:
            env.unlink()
            removed.append(str(env.relative_to(ws)))
    print(f"[strip] removed {len(removed)} path(s)")
    return removed


def main() -> int:
    if isinstance(sys.stdout, io.TextIOWrapper):
        sys.stdout.reconfigure(line_buffering=True)  # flush phase markers as they happen
    p = argparse.ArgumentParser(description="azldev tooling-evaluation harness (spike v0.1)")
    p.add_argument("--scenario", default=str(EVAL_DIR / "scenarios" / "cpio-overlay"))
    p.add_argument("--input-repo", default=str(REPO_ROOT))
    p.add_argument("--actor-model", default="claude-sonnet-5")
    p.add_argument("--evaluator-model", default="gpt-5.6-terra")
    p.add_argument("--keep-workspace", action="store_true")
    p.add_argument("--skip-evaluator", action="store_true")
    p.add_argument("--seed-only", action="store_true", help="seed + precondition then stop (harness self-check)")
    args = p.parse_args()
    try:
        return orchestrate(args)
    except HarnessError as e:
        print(f"[FATAL] {e}")
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
