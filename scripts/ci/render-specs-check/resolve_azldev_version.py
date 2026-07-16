"""Resolve the azldev version that will be present after a PR merges."""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
import time
from pathlib import Path

VERSION_RE = re.compile(r"[0-9A-Za-z._+-]+")
REPO_RE = re.compile(r"[A-Za-z0-9._-]+/[A-Za-z0-9._-]+")
SHA_RE = re.compile(r"[0-9a-f]{40}")
MERGE_ATTEMPTS = 30
CONTENT_ATTEMPTS = 5
RETRY_SECONDS = 4
PR_FIELD_COUNT = 3


class ResolutionError(RuntimeError):
    """Raised when the post-merge version cannot be resolved safely."""


class GitHubApiError(ResolutionError):
    """Raised when a GitHub API request fails."""


def validate_version(value: str, source: str) -> str:
    """Return a validated, single-token azldev version."""
    version = value.rstrip("\r\n")
    if not VERSION_RE.fullmatch(version):
        message = f"{source} is empty or contains unexpected characters"
        raise ResolutionError(message)
    return version


def read_version(path: Path) -> str:
    """Read a version file without following an untrusted symlink."""
    if path.is_symlink():
        message = f"{path} must be a regular file, not a symlink"
        raise ResolutionError(message)
    try:
        value = path.read_text(encoding="ascii")
    except (OSError, UnicodeError) as error:
        message = f"could not read {path}: {error}"
        raise ResolutionError(message) from error
    return validate_version(value, str(path))


def validate_inputs(repo: str, pr_number: int, expected_head: str) -> None:
    """Validate values used in API paths and workflow output."""
    if not REPO_RE.fullmatch(repo):
        message = f"repo is not a valid owner/repo: {repo!r}"
        raise ResolutionError(message)
    if pr_number < 1:
        message = f"pr-number is not a positive integer: {pr_number!r}"
        raise ResolutionError(message)
    if not SHA_RE.fullmatch(expected_head):
        message = f"head-sha is not a 40-character lowercase hex SHA: {expected_head!r}"
        raise ResolutionError(message)


def github_api(endpoint: str, *options: str) -> str:
    """Call GitHub through the authenticated gh CLI and return stdout."""
    command = ["gh", "api", endpoint, *options]
    result = subprocess.run(command, capture_output=True, text=True, check=False)
    if result.returncode != 0:
        detail = result.stderr.strip() or f"gh api exited with {result.returncode}"
        raise GitHubApiError(detail)
    return result.stdout


def _resolve_merge_sha(repo: str, pr_number: int, expected_head: str) -> str:
    """Poll GitHub until the current PR test-merge commit is available."""
    endpoint = f"repos/{repo}/pulls/{pr_number}"
    query = '[.head.sha, (.mergeable | tostring), (.merge_commit_sha // "")] | @tsv'
    for attempt in range(1, MERGE_ATTEMPTS + 1):
        try:
            fields = github_api(endpoint, "--jq", query).rstrip("\r\n").split("\t")
        except GitHubApiError:
            print(f"PR API call failed ({attempt}/{MERGE_ATTEMPTS}); retrying...")
        else:
            if len(fields) != PR_FIELD_COUNT:
                message = "GitHub returned an invalid PR response"
                raise ResolutionError(message)
            polled_head, mergeable, candidate = fields
            if polled_head != expected_head:
                message = f"PR head advanced from {expected_head!r} to {polled_head!r}; a newer run supersedes this one"
                raise ResolutionError(message)
            if mergeable == "false":
                message = "PR conflicts with the base branch"
                raise ResolutionError(message)
            if mergeable == "true" and SHA_RE.fullmatch(candidate):
                return candidate
            print(f"Waiting for GitHub to compute the test-merge commit ({attempt}/{MERGE_ATTEMPTS})...")
        if attempt < MERGE_ATTEMPTS:
            time.sleep(RETRY_SECONDS)

    message = f"GitHub did not publish a test-merge commit after {MERGE_ATTEMPTS} attempts"
    raise ResolutionError(message)


def _read_merged_version(repo: str, merge_sha: str) -> str:
    """Read the azldev pin from a test-merge commit."""
    endpoint = f"repos/{repo}/contents/.azldev-version?ref={merge_sha}"
    for attempt in range(1, CONTENT_ATTEMPTS + 1):
        try:
            content = github_api(endpoint, "-H", "Accept: application/vnd.github.raw")
        except GitHubApiError:
            print(f"Could not read the post-merge .azldev-version ({attempt}/{CONTENT_ATTEMPTS}); retrying...")
            if attempt < CONTENT_ATTEMPTS:
                time.sleep(RETRY_SECONDS)
        else:
            return validate_version(content, "post-merge .azldev-version")

    message = "could not read .azldev-version from the test-merge commit"
    raise ResolutionError(message)


def resolve_version(
    base_version: str,
    head_version: str,
    *,
    repo: str,
    pr_number: int,
    expected_head: str,
) -> tuple[str, bool]:
    """Return the post-merge version and whether all specs must be rendered."""
    validate_inputs(repo, pr_number, expected_head)
    base_version = validate_version(base_version, "base version")
    head_version = validate_version(head_version, "PR-head version")

    if head_version == base_version:
        return base_version, False

    merge_sha = _resolve_merge_sha(repo, pr_number, expected_head)
    merged_version = _read_merged_version(repo, merge_sha)
    return merged_version, merged_version != base_version


def parse_args() -> argparse.Namespace:
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", required=True)
    parser.add_argument("--pr-number", required=True, type=int)
    parser.add_argument("--head-sha", required=True)
    parser.add_argument("--base-version-file", required=True, type=Path)
    parser.add_argument("--head-version-file", required=True, type=Path)
    parser.add_argument("--github-output", required=True, type=Path)
    return parser.parse_args()


def main() -> int:
    """Resolve the version and write GitHub Actions step outputs."""
    args = parse_args()
    try:
        base_version = read_version(args.base_version_file)
        head_version = read_version(args.head_version_file)
        version, render_all = resolve_version(
            base_version=base_version,
            head_version=head_version,
            expected_head=args.head_sha,
            repo=args.repo,
            pr_number=args.pr_number,
        )
        with args.github_output.open("a", encoding="utf-8") as output:
            output.write(f"azldev-version={version}\n")
            output.write(f"render-all={str(render_all).lower()}\n")
        print(
            f"Resolved azldev version: {version}; render all: {str(render_all).lower()} "
            f"(base: {base_version}, PR head: {head_version})"
        )
    except ResolutionError as error:
        print(f"::error::{error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
