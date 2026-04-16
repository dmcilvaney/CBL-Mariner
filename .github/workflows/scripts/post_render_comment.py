#!/usr/bin/env python3
"""
Post (or update/delete) a PR comment with rendered-spec drift results.

Reads the JSON report produced by check_rendered_specs.py and posts a
formatted comment on the PR. Designed to run in a workflow_run context
where the base repo's GITHUB_TOKEN is available (needed for fork PRs).

Usage:
    python post_render_comment.py \\
        --report render-check-report.json \\
        --repo owner/repo \\
        --pr 123 \\
        --artifacts-url https://... \\
        --run-id 12345

Exit codes:
    0 — comment posted/updated/deleted successfully
    1 — error reading report or missing arguments

Environment:
    GH_TOKEN — required for GitHub API calls
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

COMMENT_MARKER = "<!-- RENDERED_SPEC_CHECK -->"
MAX_INLINE_DIFFS = 10
MAX_FILE_LIST = 50
MAX_COMMENT_CHARS = 60_000

# ---------------------------------------------------------------------------
# Comment formatting
# ---------------------------------------------------------------------------


def _unique_components(items: list[dict]) -> list[str]:
    seen: set[str] = set()
    out: list[str] = []
    for item in items:
        c = item["component"]
        if c not in seen:
            seen.add(c)
            out.append(c)
    out.sort()
    return out


# NOTE: _unique_components and _render_command are duplicated in check_rendered_specs.py
def _render_command(components: list[str], use_all: bool = False) -> str:
    if use_all or len(components) > 30:
        return "azldev component render -a --clean-stale"
    return f"azldev component render {' '.join(components)}"


def format_comment(
    report: dict,
    artifacts_url: str | None = None,
    run_id: str | None = None,
    repo: str | None = None,
) -> str:
    content_diffs = report.get("content_diffs", [])
    extra_files = report.get("extra_files", [])
    missing_files = report.get("missing_files", [])

    n_diff = len(content_diffs)
    n_extra = len(extra_files)
    n_missing = len(missing_files)
    total = n_diff + n_extra + n_missing

    if total == 0:
        return f"{COMMENT_MARKER}\n## ✅ Rendered specs are up to date\n"

    all_comps: list[str] = sorted(
        set(_unique_components(content_diffs) + _unique_components(missing_files))
    )
    use_all = bool(extra_files) or bool(missing_files)
    remediation_cmd = _render_command([] if use_all else all_comps, use_all=use_all)

    lines: list[str] = [
        COMMENT_MARKER,
        "## ❌ Rendered specs are out of date",
        "",
        "🚧🚧🚧🚧🚧",
        "",
        "> [!WARNING]",
        ">",
        "> **Disregard this comment.**",
        ">",
        "> Spec rendering is still under development and checked-in specs",
        "> should not be updated in PRs yet.",
        "> Please ignore this comment for now unless you are actively",
        "> working on the render pipeline.",
        "",
        "🚧🚧🚧🚧🚧",
        "",
        "**FIX:** — run this and commit the result:",
        "",
        f"```bash\n{remediation_cmd}\n```",
        "",
    ]

    if artifacts_url:
        lines.append(f"Or [download the fix patch]({artifacts_url}) and apply it:")
        lines.append("")
        if run_id and repo:
            lines.append(
                "```bash\n"
                f"gh run download {run_id} -R {repo} -n rendered-specs-patch\n"
                "git apply rendered-specs.patch\n"
                "```"
            )
        else:
            lines.append("```bash\ngit apply rendered-specs.patch\n```")
        lines.append("")

    lines.extend(
        [
            "| Category | Count |",
            "|----------|-------|",
            f"| Content diffs | {n_diff} |",
            f"| Extra files (untracked) | {n_extra} |",
            f"| Missing files (deleted) | {n_missing} |",
            "",
        ]
    )

    if content_diffs:
        lines.append("### Content diffs")
        lines.append("")
        shown = 0
        body_so_far = len("\n".join(lines))
        for item in content_diffs:
            if shown >= MAX_INLINE_DIFFS:
                remaining = n_diff - shown
                lines.append(
                    f"*… and {remaining} more file(s). "
                    "Run the remediation command above to see all changes.*"
                )
                lines.append("")
                break
            path = item["path"]
            diff_text = item.get("diff", "")
            block = (
                "<details>\n"
                f"<summary><code>{path}</code></summary>\n\n"
                f"```diff\n{diff_text}\n```\n\n"
                "</details>\n"
            )
            if body_so_far + len(block) > MAX_COMMENT_CHARS - 2000:
                remaining = n_diff - shown
                lines.append(
                    f"*… and {remaining} more file(s) — comment size limit reached. "
                    "Run the remediation command above to see all changes.*"
                )
                lines.append("")
                break
            lines.append(block)
            body_so_far += len(block)
            shown += 1

    if extra_files:
        lines.append("### Files to add")
        lines.append("")
        lines.append(
            "These files are produced by `azldev component render` but are "
            "missing from your branch. Add them."
        )
        lines.append("")
        for item in extra_files[:MAX_FILE_LIST]:
            lines.append(f"- `{item['path']}`")
        if len(extra_files) > MAX_FILE_LIST:
            lines.append(f"\n*… and {len(extra_files) - MAX_FILE_LIST} more file(s).*")
        lines.append("")

    if missing_files:
        lines.append("### Files to remove")
        lines.append("")
        lines.append(
            "These files are in your branch but are not produced by render. "
            "Remove them."
        )
        lines.append("")
        for item in missing_files[:MAX_FILE_LIST]:
            lines.append(f"- `{item['path']}`")
        if len(missing_files) > MAX_FILE_LIST:
            lines.append(
                f"\n*… and {len(missing_files) - MAX_FILE_LIST} more file(s).*"
            )
        lines.append("")

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# GitHub comment posting
# ---------------------------------------------------------------------------


def _gh(*args: str) -> str:
    return subprocess.run(
        ["gh", *args], capture_output=True, text=True, check=True
    ).stdout.strip()


def find_existing_comment(repo: str, pr: str) -> str | None:
    try:
        output = _gh(
            "api",
            "--paginate",
            f"/repos/{repo}/issues/{pr}/comments",
            "--jq",
            f'.[] | select(.body | contains("{COMMENT_MARKER}")) | .id',
        )
    except subprocess.CalledProcessError:
        return None
    comment_id = output.split("\n")[0].strip() if output else None
    return comment_id or None


def post_or_update_comment(repo: str, pr: str, body: str) -> None:
    existing_id = find_existing_comment(repo, pr)
    fd, body_path = tempfile.mkstemp(prefix="render-check-comment-", suffix=".md")
    try:
        with os.fdopen(fd, "w") as f:
            f.write(body)
        if existing_id:
            print(f"Updating existing comment {existing_id}")
            _gh(
                "api",
                "--method",
                "PATCH",
                f"/repos/{repo}/issues/comments/{existing_id}",
                "-F",
                f"body=@{body_path}",
            )
        else:
            print("Creating new comment")
            _gh("pr", "comment", pr, "--repo", repo, "--body-file", body_path)
    finally:
        Path(body_path).unlink(missing_ok=True)


def delete_comment_if_exists(repo: str, pr: str) -> None:
    existing_id = find_existing_comment(repo, pr)
    if existing_id:
        print(f"Deleting stale comment {existing_id}")
        try:
            _gh(
                "api",
                "--method",
                "DELETE",
                f"/repos/{repo}/issues/comments/{existing_id}",
            )
        except subprocess.CalledProcessError:
            print("Warning: failed to delete stale comment", file=sys.stderr)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Post rendered-spec drift results as a PR comment."
    )
    parser.add_argument(
        "--report",
        type=Path,
        required=True,
        help="Path to the JSON report from check_rendered_specs.py",
    )
    parser.add_argument("--repo", required=True, help="GitHub repo (owner/repo)")
    parser.add_argument("--pr", required=True, help="PR number")
    parser.add_argument(
        "--artifacts-url", default=None, help="Direct URL to patch artifact"
    )
    parser.add_argument("--run-id", default=None, help="GitHub Actions run ID")
    args = parser.parse_args()

    try:
        with open(args.report, encoding="utf-8") as f:
            report = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError) as exc:
        print(f"Error reading report: {exc}", file=sys.stderr)
        return 1

    total = (
        len(report.get("content_diffs", []))
        + len(report.get("extra_files", []))
        + len(report.get("missing_files", []))
    )

    try:
        if total == 0:
            delete_comment_if_exists(args.repo, args.pr)
        else:
            body = format_comment(
                report,
                artifacts_url=args.artifacts_url,
                run_id=args.run_id,
                repo=args.repo,
            )
            post_or_update_comment(args.repo, args.pr, body)
    except (subprocess.CalledProcessError, OSError) as exc:
        print(f"Warning: failed to post/update PR comment: {exc}", file=sys.stderr)

    # Write to GitHub step summary if available
    summary_file = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary_file and total > 0:
        body = format_comment(
            report,
            artifacts_url=args.artifacts_url,
            run_id=args.run_id,
            repo=args.repo,
        )
        max_summary = 1_000_000  # GH step summary limit is 1024 KiB
        if len(body) <= max_summary:
            with open(summary_file, "a", encoding="utf-8") as sf:
                sf.write(body)
                sf.write("\n")
        else:
            print("Warning: step summary too large, skipping", file=sys.stderr)

    return 0


if __name__ == "__main__":
    sys.exit(main())
