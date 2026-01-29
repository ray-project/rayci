"""Tests for wanda wheel packaging."""

import tempfile
import zipfile
from pathlib import Path
from unittest import mock

import pytest

from build_wheels import (
    PLATFORM_MAP,
    get_version_from_git,
    write_version_file,
)
from pdm_build import is_windows


class TestGetVersionFromGit:
    """Tests for get_version_from_git()."""

    def test_strips_v_prefix(self):
        """Version extraction strips the v prefix."""
        mock_result = mock.Mock()
        mock_result.stdout = "v0.27.0\n"

        with mock.patch("subprocess.run", return_value=mock_result):
            got = get_version_from_git()
            want = "0.27.0"
            assert got == want, f"get_version_from_git() = {got!r}, want {want!r}"

    def test_handles_version_without_prefix(self):
        """Version extraction handles versions without v prefix."""
        mock_result = mock.Mock()
        mock_result.stdout = "1.2.3\n"

        with mock.patch("subprocess.run", return_value=mock_result):
            got = get_version_from_git()
            want = "1.2.3"
            assert got == want, f"get_version_from_git() = {got!r}, want {want!r}"

    def test_returns_default_on_git_error(self):
        """Version extraction returns 0.0.0 on git error."""
        import subprocess

        with mock.patch(
            "subprocess.run",
            side_effect=subprocess.CalledProcessError(1, "git"),
        ):
            got = get_version_from_git()
            want = "0.0.0"
            assert got == want, f"get_version_from_git() = {got!r}, want {want!r}"


class TestWriteVersionFile:
    """Tests for write_version_file()."""

    def test_creates_version_file_with_correct_format(self):
        """write_version_file creates VERSION with correct format."""
        with tempfile.TemporaryDirectory() as tmpdir:
            script_dir = Path(tmpdir)

            mock_result = mock.Mock()
            mock_result.stdout = "v1.0.0\n"

            with mock.patch("subprocess.run", return_value=mock_result):
                got_version = write_version_file(script_dir)

            want_version = "1.0.0"
            assert (
                got_version == want_version
            ), f"write_version_file() returned {got_version!r}, want {want_version!r}"

            version_file = script_dir / "VERSION"
            assert version_file.exists(), "VERSION file not created"

            got_content = version_file.read_text()
            want_content = '__version__ = "1.0.0"\n'
            assert (
                got_content == want_content
            ), f"VERSION content = {got_content!r}, want {want_content!r}"


class TestPlatformMap:
    """Tests for PLATFORM_MAP configuration."""

    REQUIRED_KEYS = {"goos", "goarch", "platform"}

    def test_all_platforms_have_required_keys(self):
        """All platform configs have required keys."""
        for platform_key, config in PLATFORM_MAP.items():
            for required_key in self.REQUIRED_KEYS:
                assert (
                    required_key in config
                ), f"PLATFORM_MAP[{platform_key!r}] missing required key {required_key!r}"

    @pytest.mark.parametrize(
        "platform_key",
        ["darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"],
    )
    def test_expected_platforms_exist(self, platform_key):
        """Expected platforms are defined in PLATFORM_MAP."""
        assert (
            platform_key in PLATFORM_MAP
        ), f"Expected platform {platform_key!r} not in PLATFORM_MAP"


class TestWheelStructure:
    """Tests for wheel structure validation."""

    def test_wheel_contains_expected_files(self):
        """Verify wheel contains exactly the expected files."""
        script_dir = Path(__file__).parent
        dist_dir = script_dir / "dist"

        wheels = list(dist_dir.glob("*.whl"))
        if not wheels:
            pytest.skip("No wheel found in dist/ - run build first")

        wheel_path = wheels[0]
        version = wheel_path.name.split("-")[1]

        want = {
            f"wanda_bin-{version}.data/scripts/wanda",
            f"wanda_bin-{version}.dist-info/METADATA",
            f"wanda_bin-{version}.dist-info/WHEEL",
            f"wanda_bin-{version}.dist-info/RECORD",
            f"wanda_bin-{version}.dist-info/entry_points.txt",
            "wanda_bin/__init__.py",
        }

        with zipfile.ZipFile(wheel_path, "r") as whl:
            got = set(whl.namelist())

        assert (
            got == want
        ), f"wheel files mismatch:\n  got:  {sorted(got)}\n  want: {sorted(want)}"


class TestIsWindows:
    """Tests for is_windows() platform detection."""

    def test_goos_windows_returns_true(self):
        """is_windows returns True when GOOS=windows."""
        with mock.patch.dict("os.environ", {"GOOS": "windows"}):
            assert is_windows() is True

    def test_goos_linux_returns_false(self):
        """is_windows returns False when GOOS=linux."""
        with mock.patch.dict("os.environ", {"GOOS": "linux"}):
            assert is_windows() is False

    def test_no_goos_uses_sys_platform(self):
        """is_windows falls back to sys.platform when GOOS not set."""
        with mock.patch.dict("os.environ", {}, clear=True):
            with mock.patch("pdm_build.sys.platform", "win32"):
                assert is_windows() is True
            with mock.patch("pdm_build.sys.platform", "linux"):
                assert is_windows() is False
