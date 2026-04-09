#!/usr/bin/env python3
"""
Check rendered specs for drift and (optionally) post a PR comment.

Compares the committed specs/ tree against the working tree (after
`azldev comp render -a` has been run) and reports meaningful differences,
filtering out changelog-timestamp noise.

Usage:
    # Just check (local dev, CI without comment posting):
    python check_rendered_specs.py

    # Check and post/update a PR comment:
    python check_rendered_specs.py --repo owner/repo --pr 123

Exit codes:
    0 — specs are up to date (timestamp-only noise filtered)
    1 — real diffs, extra files, or missing files detected

Environment:
    GH_TOKEN — required when --repo/--pr are given
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

COMMENT_MARKER = "<!-- RENDERED_SPEC_CHECK -->"
MAX_INLINE_DIFFS = 20
MAX_COMMENT_CHARS = 60_000  # GH limit is 65 535; leave headroom

# Matches the azldev-generated changelog line and captures the date portion.
# e.g. "* Wed Apr 08 2026 azldev <azurelinux@microsoft.com> - 1.0-1"
_CHANGELOG_DATE_RE = re.compile(
    r"^\* [A-Z][a-z]{2} [A-Z][a-z]{2} [0-9]{2} [0-9]{4} azldev "
)

# ---------------------------------------------------------------------------
# Git helpers
# ---------------------------------------------------------------------------


def _git(*args: str) -> str:
    """Run a git command and return stdout."""
    result = subprocess.run(
        ["git", *args],
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout


def _git_lines(*args: str) -> list[str]:
    """Run a git command and return non-empty output lines."""
    return [line for line in _git(*args).splitlines() if line]


# ---------------------------------------------------------------------------
# Normalisation
# ---------------------------------------------------------------------------


def normalize_changelog_date(text: str) -> str:
    """Replace the date on azldev changelog entries with a placeholder."""
    lines = text.splitlines(keepends=True)
    out: list[str] = []
    for line in lines:
        if _CHANGELOG_DATE_RE.match(line):
            line = _CHANGELOG_DATE_RE.sub("* DATEPLACEHOLDER azldev ", line)
        out.append(line)
    return "".join(out)


# ---------------------------------------------------------------------------
# Diff / classification
# ---------------------------------------------------------------------------


def component_from_path(path: str) -> str:
    """Extract the component name from a specs/ path.

    specs/a/accountsservice/accountsservice.spec → accountsservice
    """
    parts = path.split("/")
    if len(parts) >= 3:
        return parts[2]
    return path


def classify_changes() -> tuple[list[str], list[str], list[str]]:
    """Return (changed, extra, missing) file lists in specs/."""
    changed = _git_lines("diff", "--name-only", "--", "specs/")
    extra = _git_lines("ls-files", "--others", "--exclude-standard", "--", "specs/")
    missing = _git_lines("ls-files", "--deleted", "--", "specs/")
    return changed, extra, missing


def filter_timestamp_noise(changed_files: list[str]) -> list[dict]:
    """Filter changed files to only those with real (non-timestamp) diffs.

    Returns a list of dicts with path, component, and the normalised diff.
    """
    real_diffs: list[dict] = []
    for path in changed_files:
        committed = _git("show", f"HEAD:{path}")
        try:
            with open(path, encoding="utf-8", errors="replace") as f:
                working = f.read()
        except FileNotFoundError:
            # File was deleted — handled in 'missing' bucket.
            continue

        norm_committed = normalize_changelog_date(committed)
        norm_working = normalize_changelog_date(working)

        if norm_committed == norm_working:
            continue  # timestamp-only noise

        # Produce a unified diff of the normalised content for the report.
        with tempfile.NamedTemporaryFile(mode="w", suffix=".spec", delete=False) as tf:
            tf.write(norm_working)
            tf_path = tf.name
        try:
            diff_result = subprocess.run(
                [
                    "diff",
                    "-u",
                    "--label",
                    f"committed/{path}",
                    "--label",
                    f"rendered/{path}",
                    "-",
                    tf_path,
                ],
                input=norm_committed,
                capture_output=True,
                text=True,
            )
            udiff = diff_result.stdout
        finally:
            os.unlink(tf_path)

        real_diffs.append(
            {
                "path": path,
                "component": component_from_path(path),
                "diff": udiff,
            }
        )

    return real_diffs


# ---------------------------------------------------------------------------
# Report building
# ---------------------------------------------------------------------------


def build_report(
    content_diffs: list[dict],
    extra_files: list[str],
    missing_files: list[str],
) -> dict:
    """Build the JSON-serialisable report."""
    return {
        "content_diffs": content_diffs,
        "extra_files": [
            {"path": p, "component": component_from_path(p)} for p in extra_files
        ],
        "missing_files": [
            {"path": p, "component": component_from_path(p)} for p in missing_files
        ],
    }


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


def _render_command(components: list[str], use_all: bool = False) -> str:
    if use_all or len(components) > 30:
        return "azldev comp render -a"
    args = " ".join(f"-p {c}" for c in components)
    return f"azldev comp render {args}"


def format_comment(report: dict) -> str:
    content_diffs = report.get("content_diffs", [])
    extra_files = report.get("extra_files", [])
    missing_files = report.get("missing_files", [])

    n_diff = len(content_diffs)
    n_extra = len(extra_files)
    n_missing = len(missing_files)
    total = n_diff + n_extra + n_missing

    if total == 0:
        return f"{COMMENT_MARKER}\n## ✅ Rendered specs are up to date\n"

    lines: list[str] = [
        COMMENT_MARKER,
        "## ❌ Rendered specs are out of date",
        "",
        "| Category | Count |",
        "|----------|-------|",
        f"| Content diffs | {n_diff} |",
        f"| Extra files (untracked) | {n_extra} |",
        f"| Missing files (deleted) | {n_missing} |",
        "",
    ]

    # --- Content diffs ---
    if content_diffs:
        lines.append("### Content diffs")
        lines.append("")
        shown = 0
        for item in content_diffs:
            if shown >= MAX_INLINE_DIFFS:
                remaining = n_diff - shown
                lines.append(
                    f"*… and {remaining} more file(s). "
                    "Run the remediation command below to see all changes.*"
                )
                lines.append("")
                break
            path = item["path"]
            diff_text = item.get("diff", "")
            lines.append("<details>")
            lines.append(f"<summary><code>{path}</code></summary>")
            lines.append("")
            lines.append("```diff")
            lines.append(diff_text)
            lines.append("```")
            lines.append("")
            lines.append("</details>")
            lines.append("")
            shown += 1

    # --- Extra files ---
    if extra_files:
        lines.append("### Extra files")
        lines.append("")
        lines.append(
            "These files were generated by `azldev comp render` but are "
            "not committed. If the component is new, commit them. Otherwise "
            "investigate why render produced unexpected output."
        )
        lines.append("")
        for item in extra_files:
            lines.append(f"- `{item['path']}`")
        lines.append("")

    # --- Missing files ---
    if missing_files:
        lines.append("### Missing files")
        lines.append("")
        lines.append(
            "These files are committed but were not produced by render. "
            "If the component was removed, delete them. Otherwise the "
            "component definition may need fixing."
        )
        lines.append("")
        for item in missing_files:
            lines.append(f"- `{item['path']}`")
        lines.append("")

    # --- Remediation ---
    lines.append("### Remediation")
    lines.append("")

    all_comps: list[str] = sorted(
        set(_unique_components(content_diffs) + _unique_components(missing_files))
    )

    if extra_files:
        lines.append(
            "Since there are extra (untracked) files, a full render is recommended:"
        )
        lines.append("")
        lines.append(f"```bash\n{_render_command([], use_all=True)}\n```")
    elif all_comps:
        lines.append("Re-render the affected component(s) and commit the result:")
        lines.append("")
        lines.append(f"```bash\n{_render_command(all_comps)}\n```")

    lines.append("")
    lines.append("Then commit the updated specs.")

    body = "\n".join(lines)

    if len(body) > MAX_COMMENT_CHARS:
        truncation_note = (
            "\n\n*Comment truncated — too many diffs to display. "
            "Run the render command locally to see all changes.*"
        )
        body = body[: MAX_COMMENT_CHARS - len(truncation_note)] + truncation_note

    return body


# ---------------------------------------------------------------------------
# GitHub comment posting
# ---------------------------------------------------------------------------


def _gh(*args: str) -> str:
    result = subprocess.run(
        ["gh", *args],
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout.strip()


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
    if existing_id:
        print(f"Updating existing comment {existing_id}")
        _gh(
            "api",
            "--method",
            "PATCH",
            f"/repos/{repo}/issues/comments/{existing_id}",
            "-f",
            f"body={body}",
        )
    else:
        print("Creating new comment")
        _gh("pr", "comment", pr, "--repo", repo, "--body", body)


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
        description="Check rendered specs for drift and optionally post a PR comment."
    )
    parser.add_argument(
        "--report",
        type=Path,
        default=None,
        help="Write JSON report to this path",
    )
    parser.add_argument("--repo", default=None, help="GitHub repo (owner/repo)")
    parser.add_argument("--pr", default=None, help="PR number")
    args = parser.parse_args()

    # 1. Classify changes
    changed, extra, missing = classify_changes()
    print(
        f"Raw counts: changed={len(changed)} extra={len(extra)} missing={len(missing)}"
    )

    # 2. Filter timestamp noise from content diffs
    content_diffs = filter_timestamp_noise(changed)
    print(f"After timestamp filtering: {len(content_diffs)} real content diff(s)")

    # 3. Build report
    report = build_report(content_diffs, extra, missing)

    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        with open(args.report, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
        print(f"Report written to {args.report}")

    total = len(content_diffs) + len(extra) + len(missing)

    # 4. Post comment or clean up
    if args.repo and args.pr:
        if total == 0:
            delete_comment_if_exists(args.repo, args.pr)
        else:
            body = format_comment(report)
            post_or_update_comment(args.repo, args.pr, body)

    # 5. Write to GitHub step summary if available
    summary_file = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary_file and total > 0:
        body = format_comment(report)
        with open(summary_file, "a", encoding="utf-8") as sf:
            sf.write(body)
            sf.write("\n")

    # 6. Print summary and exit
    if total == 0:
        print("All rendered specs are up to date (timestamp-only noise filtered).")
        return 0

    print(
        f"::error::{len(content_diffs)} content diff(s), "
        f"{len(extra)} extra file(s), {len(missing)} missing file(s)"
    )

    # Print compact remediation to the log too
    all_comps = sorted(
        set(
            _unique_components(content_diffs)
            + _unique_components(report.get("missing_files", []))
        )
    )
    if extra:
        print("Remediation: azldev comp render -a")
    elif all_comps:
        print(f"Remediation: {_render_command(all_comps)}")

    return 1


if __name__ == "__main__":
    sys.exit(main())
