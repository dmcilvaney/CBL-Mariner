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
    github_api = Mock(return_value=f"{'a' * 40}\t{'b' * 40}\ttrue\n")
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


def test_read_version_uses_selected_azldev_module(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    """Read the selected azldev module version through Go."""
    modfile = tmp_path / "go.mod"
    modfile.write_text("module example.com/test\n", encoding="ascii")
    run = Mock(
        return_value=subprocess.CompletedProcess(
            [],
            0,
            stdout="v1.2.3\n",
            stderr="",
        )
    )
    monkeypatch.setattr(resolver.subprocess, "run", run)

    if resolver.read_version(modfile) != "v1.2.3":
        pytest.fail("unexpected azldev tool version")
    command = run.call_args.args[0]
    if command[:2] != ["go", "list"] or "-mod=readonly" not in command:
        pytest.fail(f"unexpected Go command: {command}")


def test_resolve_hash_validates_module_origin(monkeypatch: pytest.MonkeyPatch) -> None:
    """Resolve a selected module version to its full Microsoft repository hash."""
    revision = "c" * 40
    run = Mock(
        return_value=subprocess.CompletedProcess(
            [],
            0,
            stdout=(
                '{"Origin":{"VCS":"git",'
                '"URL":"https://github.com/microsoft/azure-linux-dev-tools",'
                f'"Hash":"{revision}"}}}}'
            ),
            stderr="",
        )
    )
    monkeypatch.setattr(resolver.subprocess, "run", run)

    if resolver.resolve_hash("v1.2.3") != revision:
        pytest.fail("unexpected resolved hash")


def test_hash_cli_emits_resolved_hash(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """Print the hash resolved from the module tool requirement."""
    revision = "d" * 40
    monkeypatch.setattr(resolver, "read_version", Mock(return_value="v1.2.3"))
    monkeypatch.setattr(resolver, "resolve_hash", Mock(return_value=revision))
    monkeypatch.setattr(
        sys,
        "argv",
        ["resolve_azldev_version.py", "hash", str(tmp_path / "go.mod")],
    )

    if resolver.main() != 0:
        pytest.fail("hash resolver unexpectedly failed")
    if capsys.readouterr().out != f"{revision}\n":
        pytest.fail("hash resolver emitted unexpected output")


def test_hash_cli_rejects_multiline_go_output(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """Reject an injected output record in Go's selected module version."""
    modfile = tmp_path / "go.mod"
    modfile.write_text("module example.com/test\n", encoding="ascii")
    injected_hash = "e" * 40
    monkeypatch.setattr(
        resolver.subprocess,
        "run",
        Mock(
            return_value=subprocess.CompletedProcess(
                [],
                0,
                stdout=f"v1.2.3\nazldev-hash={injected_hash}\n",
                stderr="",
            )
        ),
    )
    monkeypatch.setattr(
        sys,
        "argv",
        ["resolve_azldev_version.py", "hash", str(modfile)],
    )

    if resolver.main() == 0:
        pytest.fail("multiline Go output unexpectedly succeeded")
    if capsys.readouterr().out:
        pytest.fail("resolver emitted machine-readable output after rejecting Go output")


def test_differing_versions_use_test_merge_version(monkeypatch: pytest.MonkeyPatch) -> None:
    """Use the test-merge pin when the base and PR-head pins differ."""
    github_api = Mock(
        side_effect=[
            f"{'a' * 40}\t{'c' * 40}\tnull\n",
            f"{'a' * 40}\t{'c' * 40}\ttrue\n",
            "v2.0.0\n",
        ]
    )
    monkeypatch.setattr(resolver, "github_api", github_api)
    monkeypatch.setattr(resolver, "_version_from_content", Mock(return_value="v2.0.0"))
    monkeypatch.setattr(resolver.time, "sleep", Mock())

    version, render_all = resolver.resolve_version(
        "v1.2.3",
        "v2.0.0",
        pull_request=pull_request(base_sha="c" * 40),
    )

    if version != "v2.0.0" or not render_all:
        pytest.fail(f"unexpected resolution: version={version!r}, render_all={render_all!r}")
    content_endpoint = github_api.call_args_list[-1].args[0]
    if content_endpoint != "repos/microsoft/azurelinux/contents/go.mod?ref=refs/pull/1/merge":
        pytest.fail(f"unexpected merge-ref endpoint: {content_endpoint}")


def test_behind_base_uses_base_version_without_full_render(monkeypatch: pytest.MonkeyPatch) -> None:
    """Avoid a full render when the PR merely has an older pin than base."""
    github_api = Mock(side_effect=[f"{'a' * 40}\t{'c' * 40}\ttrue\n", "v2.0.0\n"])
    monkeypatch.setattr(resolver, "github_api", github_api)
    monkeypatch.setattr(resolver, "_version_from_content", Mock(return_value="v2.0.0"))

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
    github_api = Mock(return_value=f"{'a' * 40}\t{'c' * 40}\ttrue\n")
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


def test_github_api_pins_current_rest_version(monkeypatch: pytest.MonkeyPatch) -> None:
    """Request the REST contract used by the merge-ref implementation."""
    result = subprocess.CompletedProcess([], 0, stdout="ok", stderr="")
    run = Mock(return_value=result)
    monkeypatch.setattr(resolver.subprocess, "run", run)

    if resolver.github_api("repos/microsoft/azurelinux/pulls/1") != "ok":
        pytest.fail("unexpected API output")
    command = run.call_args.args[0]
    expected_header = f"X-GitHub-Api-Version: {resolver.GITHUB_API_VERSION}"
    if expected_header not in command:
        pytest.fail(f"API version header missing from command: {command}")


def test_main_rejects_module_without_azldev_requirement(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """Reject a module without an azldev requirement before emitting output."""
    base_file = tmp_path / "base.mod"
    head_file = tmp_path / "head.mod"
    base_file.write_text("module example.com/base\n", encoding="ascii")
    head_file.write_text("module example.com/head\n", encoding="ascii")
    github_api = Mock()
    monkeypatch.setattr(resolver, "github_api", github_api)
    monkeypatch.setattr(
        resolver,
        "_run_go",
        Mock(side_effect=resolver.ResolutionError("azldev is not a known dependency")),
    )
    monkeypatch.setattr(
        sys,
        "argv",
        [
            "resolve_azldev_version.py",
            "post-merge",
            "--repo",
            "microsoft/azurelinux",
            "--pr-number",
            "1",
            "--head-sha",
            "a" * 40,
            "--base-sha",
            "b" * 40,
            "--base-modfile",
            str(base_file),
            "--head-modfile",
            str(head_file),
        ],
    )

    if resolver.main() == 0:
        pytest.fail("module without azldev requirement unexpectedly succeeded")
    if capsys.readouterr().out:
        pytest.fail("resolver emitted machine-readable output after rejecting the version")
    github_api.assert_not_called()


def test_read_version_rejects_symlink(tmp_path: Path) -> None:
    """Reject a go.mod symlink before parsing its target."""
    target = tmp_path / "target"
    link = tmp_path / "version"
    target.write_text("v1.2.3\n", encoding="ascii")
    link.symlink_to(target)

    with pytest.raises(resolver.ResolutionError, match="symlink"):
        resolver.read_version(link)
