#!/usr/bin/env python3
"""
Check rendered specs for drift.

Compares the committed specs tree against the working tree (after
`azldev component render -a` has been run) and reports meaningful differences,
filtering out changelog-timestamp noise.

Usage:
    python check_rendered_specs.py --specs-dir specs
    python check_rendered_specs.py --specs-dir specs --report report.json --patch fix.patch

Exit codes:
    0 — specs are up to date (timestamp-only noise filtered)
    1 — real diffs, extra files, or missing files detected
"""

from __future__ import annotations

import argparse
import difflib
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

# Matches the azldev-generated changelog line.
# e.g. "* Wed Apr 08 2026 azldev <> - 1.0-1"
_CHANGELOG_DATE_RE = re.compile(
    r"^\* [A-Z][a-z]{2} [A-Z][a-z]{2} [0-9]{2} [0-9]{4} azldev "
)

# ---------------------------------------------------------------------------
# Git helpers
# ---------------------------------------------------------------------------


def _git(*args: str) -> str:
    """Run a git command and return stdout."""
    return subprocess.run(
        ["git", *args], capture_output=True, text=True, check=True
    ).stdout


def _git_lines(*args: str) -> list[str]:
    """Run a git command and return non-empty output lines."""
    return [line for line in _git(*args).splitlines() if line]


# ---------------------------------------------------------------------------
# Normalisation
# ---------------------------------------------------------------------------


def normalize_changelog_date(text: str) -> str:
    """Replace the date on azldev changelog entries with a placeholder."""
    out: list[str] = []
    for line in text.splitlines(keepends=True):
        if _CHANGELOG_DATE_RE.match(line):
            line = _CHANGELOG_DATE_RE.sub("* DATEPLACEHOLDER azldev ", line)
        out.append(line)
    return "".join(out)


# ---------------------------------------------------------------------------
# Diff / classification
# ---------------------------------------------------------------------------


def component_from_path(file_path: str) -> str:
    """Extract the component name from a specs path.

    The component is always the direct parent directory:
    specs/a/acl/acl.spec → acl
    /abs/path/specs/n/nano/nano.spec → nano
    """
    return Path(file_path).parent.name


def classify_changes(specs_dir: Path) -> tuple[list[str], list[str], list[str]]:
    """Return (changed, extra, missing) file lists under specs_dir.

    The three lists are disjoint: changed contains only modified files,
    missing contains only deleted files, and extra contains untracked files.
    """
    sd = str(specs_dir)
    changed = _git_lines("diff", "--diff-filter=M", "--name-only", "--", sd)
    extra = _git_lines("ls-files", "--others", "--exclude-standard", "--", sd)
    missing = _git_lines("ls-files", "--deleted", "--", sd)
    return changed, extra, missing


def filter_timestamp_noise(changed_files: list[str], specs_dir: Path) -> list[dict]:
    """Filter changed files to only those with real (non-timestamp) diffs.

    Only .spec files are checked for timestamp noise — other file types
    (patches, GPG keys, etc.) are always treated as real changes.
    """
    real_diffs: list[dict] = []
    for path_str in changed_files:
        file_path = Path(path_str)
        is_spec = file_path.suffix == ".spec"

        # Read committed version — use bytes to handle binary files
        try:
            committed_bytes = subprocess.run(
                ["git", "show", f"HEAD:{path_str}"],
                capture_output=True,
                check=True,
            ).stdout
        except subprocess.CalledProcessError:
            continue

        # Try to decode as UTF-8; if it fails, it's binary — always a real diff
        try:
            committed = committed_bytes.decode("utf-8")
        except UnicodeDecodeError:
            real_diffs.append(
                {
                    "path": path_str,
                    "component": component_from_path(path_str),
                    "diff": f"Binary file {path_str} differs",
                }
            )
            continue

        try:
            working = file_path.read_text(encoding="utf-8", errors="replace")
        except FileNotFoundError:
            continue

        if is_spec:
            norm_committed = normalize_changelog_date(committed)
            norm_working = normalize_changelog_date(working)
        else:
            norm_committed = committed
            norm_working = working

        # Equality check on the *normalised* text filters out timestamp-only
        # drift (the whole point of this function). If the normalised
        # versions match, skip.
        if norm_committed == norm_working:
            continue

        # Use the original diff for display purposes.
        udiff = "".join(
            difflib.unified_diff(
                committed.splitlines(keepends=True),
                working.splitlines(keepends=True),
                fromfile=f"committed/{path_str}",
                tofile=f"rendered/{path_str}",
            )
        )

        real_diffs.append(
            {
                "path": path_str,
                "component": component_from_path(path_str),
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


# NOTE: _unique_components and _render_command are duplicated in post_render_comment.py
def _render_command(components: list[str], use_all: bool = False) -> str:
    if use_all or len(components) > 30:
        return "azldev component render -a --clean-stale"
    return f"azldev component render {' '.join(components)}"


def generate_patch(
    content_diffs: list[dict],
    extra_files: list[str],
    missing_files: list[str],
) -> bytes:
    """Generate a git patch covering all detected drift.

    Uses `git add -N` to mark untracked (extra) files as intent-to-add,
    then runs `git diff` on the affected files to capture modified, new,
    and deleted files in one clean patch. Uses --pathspec-from-file to
    avoid ARG_MAX limits with large component counts.
    """
    paths = [d["path"] for d in content_diffs] + extra_files + missing_files
    if not paths:
        return b""

    # Write paths to temp files outside CWD (which may be an untrusted
    # PR checkout where symlinks could redirect writes).
    pathspec_fd, pathspec_path = tempfile.mkstemp(prefix="render-check-", suffix=".txt")
    with os.fdopen(pathspec_fd, "w") as f:
        f.write("\n".join(paths))

    # Mark untracked files as intent-to-add so git diff includes them
    extra_pathspec_path = None
    if extra_files:
        extra_fd, extra_pathspec_path = tempfile.mkstemp(
            prefix="render-check-extra-", suffix=".txt"
        )
        with os.fdopen(extra_fd, "w") as f:
            f.write("\n".join(extra_files))
        try:
            subprocess.run(
                ["git", "add", "-N", "--pathspec-from-file", extra_pathspec_path],
                check=True,
                capture_output=True,
            )
        except subprocess.CalledProcessError:
            pass

    try:
        result = subprocess.run(
            ["git", "diff", "--pathspec-from-file", pathspec_path],
            capture_output=True,
            check=True,
        )
        patch = result.stdout
    except subprocess.CalledProcessError:
        patch = b""
    finally:
        Path(pathspec_path).unlink(missing_ok=True)

    # Undo the intent-to-add so we don't leave index dirty
    if extra_pathspec_path:
        try:
            subprocess.run(
                ["git", "reset", "--pathspec-from-file", extra_pathspec_path],
                check=True,
                capture_output=True,
            )
        except subprocess.CalledProcessError:
            pass
        finally:
            Path(extra_pathspec_path).unlink(missing_ok=True)

    return patch


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Check rendered specs for drift. Outputs a JSON report and optional patch."
    )
    parser.add_argument(
        "--specs-dir",
        type=Path,
        required=True,
        help="Path to the rendered specs directory",
    )
    parser.add_argument(
        "--report",
        type=Path,
        default=None,
        help="Write JSON report to this path",
    )
    parser.add_argument(
        "--patch",
        type=Path,
        default=None,
        help="Write a .patch file for all detected drift",
    )
    args = parser.parse_args()

    specs_dir = args.specs_dir

    # 1. Classify changes
    changed, extra, missing = classify_changes(specs_dir)
    print(
        f"Raw counts: changed={len(changed)} extra={len(extra)} missing={len(missing)}"
    )

    # 2. Filter timestamp noise from content diffs
    content_diffs = filter_timestamp_noise(changed, specs_dir)
    print(f"After timestamp filtering: {len(content_diffs)} real content diff(s)")

    # 3. Build report
    report = build_report(content_diffs, extra, missing)

    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        with open(args.report, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
        print(f"Report written to {args.report}")

    total = len(content_diffs) + len(extra) + len(missing)

    # 4. Generate patch for all drift
    if args.patch and total > 0:
        patch_content = generate_patch(content_diffs, extra, missing)
        if patch_content:
            args.patch.parent.mkdir(parents=True, exist_ok=True)
            args.patch.write_bytes(patch_content)
            print(f"Patch written to {args.patch}")

    # 5. Print summary and exit
    if total == 0:
        print("All rendered specs are up to date (timestamp-only noise filtered).")
        return 0

    print(
        f"::error::{len(content_diffs)} content diff(s), "
        f"{len(extra)} extra file(s), {len(missing)} missing file(s)"
    )
    all_comps = sorted(
        set(
            _unique_components(content_diffs)
            + _unique_components(report.get("missing_files", []))
        )
    )
    if extra or missing:
        print(f"Remediation: {_render_command([], use_all=True)}")
    elif all_comps:
        print(f"Remediation: {_render_command(all_comps)}")

    return 1


if __name__ == "__main__":
    sys.exit(main())
