"""Tests for post-merge azldev version resolution."""

from __future__ import annotations

import subprocess
import sys
from typing import TYPE_CHECKING
from unittest.mock import Mock

import pytest
import resolve_azldev_version as resolver

if TYPE_CHECKING:
    from pathlib import Path


def pull_request(*, number: int = 1, head_sha: str = "a" * 40, base_sha: str = "b" * 40) -> resolver.PullRequest:
    """Build a test pull request snapshot."""
    return resolver.PullRequest(
        repo="microsoft/azurelinux",
        number=number,
        head_sha=head_sha,
        base_sha=base_sha,
    )


def test_equal_versions_skip_merge_poll(monkeypatch: pytest.MonkeyPatch) -> None:
    """Validate one PR snapshot without polling for a test-merge commit."""
    github_api = Mock(return_value=f"{'a' * 40}\t{'b' * 40}\ttrue\t\n")
    monkeypatch.setattr(resolver, "github_api", github_api)

    version, render_all = resolver.resolve_version(
        "v1.2.3",
        "v1.2.3",
        pull_request=pull_request(),
    )

    if version != "v1.2.3" or render_all:
        pytest.fail(f"unexpected resolution: version={version!r}, render_all={render_all!r}")
    if github_api.call_count != 1:
        pytest.fail(f"expected one PR snapshot request, got {github_api.call_count}")


def test_differing_versions_use_test_merge_version(monkeypatch: pytest.MonkeyPatch) -> None:
    """Use the test-merge pin when the base and PR-head pins differ."""
    github_api = Mock(
        side_effect=[
            f"{'a' * 40}\t{'c' * 40}\ttrue\t\n",
            f"{'a' * 40}\t{'c' * 40}\ttrue\t{'b' * 40}\n",
            "v2.0.0\n",
        ]
    )
    monkeypatch.setattr(resolver, "github_api", github_api)
    monkeypatch.setattr(resolver.time, "sleep", Mock())

    version, render_all = resolver.resolve_version(
        "v1.2.3",
        "v2.0.0",
        pull_request=pull_request(base_sha="c" * 40),
    )

    if version != "v2.0.0" or not render_all:
        pytest.fail(f"unexpected resolution: version={version!r}, render_all={render_all!r}")


def test_behind_base_uses_base_version_without_full_render(monkeypatch: pytest.MonkeyPatch) -> None:
    """Avoid a full render when the PR merely has an older pin than base."""
    github_api = Mock(side_effect=[f"{'a' * 40}\t{'c' * 40}\ttrue\t{'b' * 40}\n", "v2.0.0\n"])
    monkeypatch.setattr(resolver, "github_api", github_api)

    version, render_all = resolver.resolve_version(
        "v2.0.0",
        "v1.2.3",
        pull_request=pull_request(base_sha="c" * 40),
    )

    if version != "v2.0.0" or render_all:
        pytest.fail(f"unexpected resolution: version={version!r}, render_all={render_all!r}")


@pytest.mark.parametrize(
    ("pr_number", "expected_head", "expected_base", "message"),
    [
        (0, "a" * 40, "b" * 40, "positive integer"),
        (1, "not-a-sha\n", "b" * 40, "head-sha.*40-character lowercase hex SHA"),
        (1, "a" * 40, "not-a-sha\n", "base-sha.*40-character lowercase hex SHA"),
    ],
)
def test_invalid_inputs_fail_before_github(
    monkeypatch: pytest.MonkeyPatch,
    pr_number: int,
    expected_head: str,
    expected_base: str,
    message: str,
) -> None:
    """Reject malformed reusable-workflow inputs before calling GitHub."""
    github_api = Mock()
    monkeypatch.setattr(resolver, "github_api", github_api)

    with pytest.raises(resolver.ResolutionError, match=message):
        resolver.resolve_version(
            "v1.2.3",
            "v1.2.3",
            pull_request=pull_request(
                number=pr_number,
                head_sha=expected_head,
                base_sha=expected_base,
            ),
        )

    github_api.assert_not_called()


def test_stale_base_fails_before_equal_pin_result(monkeypatch: pytest.MonkeyPatch) -> None:
    """Reject a live PR base that differs from the trusted checkout."""
    github_api = Mock(return_value=f"{'a' * 40}\t{'c' * 40}\ttrue\t\n")
    monkeypatch.setattr(resolver, "github_api", github_api)

    with pytest.raises(resolver.ResolutionError, match="PR base advanced"):
        resolver.resolve_version(
            "v1.2.3",
            "v1.2.3",
            pull_request=pull_request(),
        )


def test_github_api_timeout_is_retryable(monkeypatch: pytest.MonkeyPatch) -> None:
    """Translate a hung gh process into the resolver's retryable error."""
    run = Mock(side_effect=subprocess.TimeoutExpired(["gh", "api"], 1))
    monkeypatch.setattr(resolver.subprocess, "run", run)

    with pytest.raises(resolver.GitHubApiError, match="timed out"):
        resolver.github_api("repos/microsoft/azurelinux/pulls/1")


def test_main_rejects_multiline_output_injection(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """Reject a multiline pin without appending attacker-controlled outputs."""
    base_file = tmp_path / "base-version"
    head_file = tmp_path / "head-version"
    output_file = tmp_path / "github-output"
    base_file.write_text("v1.2.3\n", encoding="ascii")
    head_file.write_text("safe\nazldev-version=attacker-value\n", encoding="ascii")
    output_file.write_text("existing-output=true\n", encoding="ascii")
    github_api = Mock()
    monkeypatch.setattr(resolver, "github_api", github_api)
    monkeypatch.setattr(
        sys,
        "argv",
        [
            "resolve_azldev_version.py",
            "--repo",
            "microsoft/azurelinux",
            "--pr-number",
            "1",
            "--head-sha",
            "a" * 40,
            "--base-sha",
            "b" * 40,
            "--base-version-file",
            str(base_file),
            "--head-version-file",
            str(head_file),
            "--github-output",
            str(output_file),
        ],
    )

    if resolver.main() == 0:
        pytest.fail("multiline version unexpectedly succeeded")
    if output_file.read_text(encoding="ascii") != "existing-output=true\n":
        pytest.fail("resolver modified GitHub output after rejecting the version")
    github_api.assert_not_called()


def test_read_version_rejects_symlink(tmp_path: Path) -> None:
    """Reject a version-file symlink before reading its target."""
    target = tmp_path / "target"
    link = tmp_path / "version"
    target.write_text("v1.2.3\n", encoding="ascii")
    link.symlink_to(target)

    with pytest.raises(resolver.ResolutionError, match="symlink"):
        resolver.read_version(link)
