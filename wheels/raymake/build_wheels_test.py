"""Tests for raymake wheel packaging."""

import tempfile
import zipfile
from pathlib import Path

import pytest

from build_wheels import (
    PLATFORM_MAP,
    read_version,
    write_version_file,
)


class TestReadVersion:
    """Tests for read_version()."""

    def test_reads_checked_in_version(self):
        """Version comes from the VERSION file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            script_dir = Path(tmpdir)
            (script_dir / "VERSION").write_text('__version__ = "0.47.0"\n')

            got = read_version(script_dir)
            want = "0.47.0"
            assert got == want, f"read_version() = {got!r}, want {want!r}"

    def test_raises_when_version_undeclared(self):
        """A VERSION file without __version__ is an error, not a default."""
        with tempfile.TemporaryDirectory() as tmpdir:
            script_dir = Path(tmpdir)
            (script_dir / "VERSION").write_text("0.47.0\n")

            with pytest.raises(RuntimeError):
                read_version(script_dir)

    def test_raises_when_version_file_missing(self):
        """A missing VERSION file is an error, not a default."""
        with tempfile.TemporaryDirectory() as tmpdir:
            with pytest.raises(FileNotFoundError):
                read_version(Path(tmpdir))

    def test_reads_the_repo_version(self):
        """The checked-in VERSION file parses."""
        got = read_version(Path(__file__).parent)
        assert got, "read_version() returned an empty version"


class TestWriteVersionFile:
    """Tests for write_version_file()."""

    def test_creates_version_file_with_correct_format(self):
        """write_version_file creates VERSION with correct format."""
        with tempfile.TemporaryDirectory() as tmpdir:
            script_dir = Path(tmpdir)

            got_version = write_version_file(script_dir, "1.0.0")

            want_version = "1.0.0"
            assert got_version == want_version, (
                f"write_version_file() returned {got_version!r}, want {want_version!r}"
            )

            version_file = script_dir / "VERSION"
            assert version_file.exists(), "VERSION file not created"

            got_content = version_file.read_text()
            want_content = '__version__ = "1.0.0"\n'
            assert got_content == want_content, (
                f"VERSION content = {got_content!r}, want {want_content!r}"
            )

    def test_round_trips_with_read_version(self):
        """What write_version_file writes, read_version reads."""
        with tempfile.TemporaryDirectory() as tmpdir:
            script_dir = Path(tmpdir)
            write_version_file(script_dir, "2.3.4")

            got = read_version(script_dir)
            want = "2.3.4"
            assert got == want, f"read_version() = {got!r}, want {want!r}"


class TestPlatformMap:
    """Tests for PLATFORM_MAP configuration."""

    REQUIRED_KEYS = {"goos", "goarch", "platform"}

    def test_all_platforms_have_required_keys(self):
        """All platform configs have required keys."""
        for platform_key, config in PLATFORM_MAP.items():
            for required_key in self.REQUIRED_KEYS:
                assert required_key in config, (
                    f"PLATFORM_MAP[{platform_key!r}] missing required key {required_key!r}"
                )

    @pytest.mark.parametrize(
        "platform_key",
        ["darwin-arm64", "linux-amd64", "linux-arm64"],
    )
    def test_expected_platforms_exist(self, platform_key):
        """Expected platforms are defined in PLATFORM_MAP."""
        assert platform_key in PLATFORM_MAP, (
            f"Expected platform {platform_key!r} not in PLATFORM_MAP"
        )


class TestWheelStructure:
    """Tests for wheel structure validation."""

    def test_wheel_contains_expected_files(self):
        """Verify wheel contains exactly the expected files."""
        script_dir = Path(__file__).parent
        dist_dir = script_dir / "dist"

        wheels = list(dist_dir.glob("*.whl"))
        if not wheels:
            pytest.skip("No wheel found in dist/ - run build first")
        assert len(wheels) == 1, f"Expected 1 wheel in dist/, found {len(wheels)}"
        wheel_path = wheels[0]
        version = wheel_path.name.split("-")[1]

        want = {
            f"raymake-{version}.data/scripts/raymake",
            f"raymake-{version}.dist-info/METADATA",
            f"raymake-{version}.dist-info/WHEEL",
            f"raymake-{version}.dist-info/RECORD",
            f"raymake-{version}.dist-info/entry_points.txt",
            "raymake/__init__.py",
        }

        with zipfile.ZipFile(wheel_path, "r") as whl:
            got = set(whl.namelist())

        # Verify required files are present (allow additional metadata files)
        missing = want - got
        assert not missing, f"wheel missing required files: {sorted(missing)}"

        # Check no unexpected scripts exist
        unexpected_scripts = [f for f in got if "/scripts/" in f and f not in want]
        assert not unexpected_scripts, (
            f"unexpected scripts in wheel: {unexpected_scripts}"
        )
