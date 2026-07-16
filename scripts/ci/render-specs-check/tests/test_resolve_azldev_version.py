"""Tests for post-merge azldev version resolution."""

from __future__ import annotations

from unittest.mock import Mock

import pytest
import resolve_azldev_version as resolver


def test_equal_versions_skip_github(monkeypatch: pytest.MonkeyPatch) -> None:
    """Avoid GitHub API calls when base and PR-head pins match."""
    github_api = Mock()
    monkeypatch.setattr(resolver, "github_api", github_api)

    version, render_all = resolver.resolve_version(
        "v1.2.3",
        "v1.2.3",
        repo="microsoft/azurelinux",
        pr_number=1,
        expected_head="a" * 40,
    )

    if version != "v1.2.3" or render_all:
        pytest.fail(f"unexpected resolution: version={version!r}, render_all={render_all!r}")
    github_api.assert_not_called()


def test_differing_versions_use_test_merge_version(monkeypatch: pytest.MonkeyPatch) -> None:
    """Use the test-merge pin when the base and PR-head pins differ."""
    github_api = Mock(side_effect=[f"{'a' * 40}\ttrue\t\n", f"{'a' * 40}\ttrue\t{'b' * 40}\n", "v2.0.0\n"])
    monkeypatch.setattr(resolver, "github_api", github_api)
    monkeypatch.setattr(resolver.time, "sleep", Mock())

    version, render_all = resolver.resolve_version(
        "v1.2.3",
        "v2.0.0",
        repo="microsoft/azurelinux",
        pr_number=1,
        expected_head="a" * 40,
    )

    if version != "v2.0.0" or not render_all:
        pytest.fail(f"unexpected resolution: version={version!r}, render_all={render_all!r}")


def test_behind_base_uses_base_version_without_full_render(monkeypatch: pytest.MonkeyPatch) -> None:
    """Avoid a full render when the PR merely has an older pin than base."""
    github_api = Mock(side_effect=[f"{'a' * 40}\ttrue\t{'b' * 40}\n", "v2.0.0\n"])
    monkeypatch.setattr(resolver, "github_api", github_api)

    version, render_all = resolver.resolve_version(
        "v2.0.0",
        "v1.2.3",
        repo="microsoft/azurelinux",
        pr_number=1,
        expected_head="a" * 40,
    )

    if version != "v2.0.0" or render_all:
        pytest.fail(f"unexpected resolution: version={version!r}, render_all={render_all!r}")


@pytest.mark.parametrize(
    ("pr_number", "expected_head", "message"),
    [
        (0, "a" * 40, "positive integer"),
        (1, "not-a-sha\n", "40-character lowercase hex SHA"),
    ],
)
def test_invalid_inputs_fail_before_github(
    monkeypatch: pytest.MonkeyPatch,
    pr_number: int,
    expected_head: str,
    message: str,
) -> None:
    """Reject malformed reusable-workflow inputs before calling GitHub."""
    github_api = Mock()
    monkeypatch.setattr(resolver, "github_api", github_api)

    with pytest.raises(resolver.ResolutionError, match=message):
        resolver.resolve_version(
            "v1.2.3",
            "v1.2.3",
            repo="microsoft/azurelinux",
            pr_number=pr_number,
            expected_head=expected_head,
        )

    github_api.assert_not_called()
