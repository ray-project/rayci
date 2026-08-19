#!/usr/bin/env python3
# /// script
# requires-python = ">=3.9"
# ///
"""Build raymake wheels for all supported platforms.

Usage:
    # Build all platforms (default)
    uv run pypi/raymake/build_wheels.py

    # Build specific platform
    uv run pypi/raymake/build_wheels.py --platform darwin-arm64

    # Package a version other than the one in VERSION
    uv run pypi/raymake/build_wheels.py --version 0.48.0
"""

import argparse
import os
import re
import shutil
import subprocess
from pathlib import Path

VERSION_PATTERN = re.compile(r'^__version__ = "([^"]+)"$', re.MULTILINE)


def read_version(script_dir: Path) -> str:
    """Read the version to package from the checked-in VERSION file.

    The version is checked in rather than taken from `git describe --tags`, which is
    where it used to come from. The release build runs on an untagged commit, so
    describe answered with the *previous* tag: every wheel published under v0.47.0
    declares 0.46.0, and v0.45.0 shipped 0.44.0. Bump VERSION in the pull request
    that cuts the release.
    """
    version_file = script_dir / "VERSION"
    match = VERSION_PATTERN.search(version_file.read_text(encoding="utf-8"))
    if match is None:
        raise RuntimeError(f"{version_file} does not declare __version__")
    return match.group(1)


def write_version_file(script_dir: Path, version: str) -> str:
    """Write VERSION, for building a version other than the checked-in one."""
    version_file = script_dir / "VERSION"
    version_file.write_text(f'__version__ = "{version}"\n')
    print(f"Wrote VERSION file: {version}")
    return version


PLATFORM_MAP = {
    "darwin-arm64": {
        "goos": "darwin",
        "goarch": "arm64",
        "platform": "macosx_12_0_arm64",
    },
    "linux-amd64": {
        "goos": "linux",
        "goarch": "amd64",
        "platform": "manylinux2014_x86_64",
    },
    "linux-arm64": {
        "goos": "linux",
        "goarch": "arm64",
        "platform": "manylinux2014_aarch64",
    },
}


def build_wheel(platform_key: str, output_dir: Path) -> Path:
    """Build a wheel for the specified platform."""
    if platform_key not in PLATFORM_MAP:
        raise ValueError(
            f"Unknown platform: {platform_key}. Valid: {list(PLATFORM_MAP.keys())}"
        )

    config = PLATFORM_MAP[platform_key]
    goos = config["goos"]
    goarch = config["goarch"]
    platform_tag = config["platform"]

    print(f"\n{'=' * 60}")
    print(f"Building wheel for {platform_key}")
    print(f"  GOOS={goos} GOARCH={goarch}")
    print(f"  Platform tag: {platform_tag}")
    print(f"{'=' * 60}\n")

    # Set environment for cross-compilation
    env = os.environ.copy()
    env["GOOS"] = goos
    env["GOARCH"] = goarch
    env["CGO_ENABLED"] = "0"

    # Get the pypi/raymake directory
    script_dir = Path(__file__).parent
    dist_dir = script_dir / "dist"

    # Clean dist directory for this build
    if dist_dir.exists():
        shutil.rmtree(dist_dir)

    # Build the wheel
    args = ["uv", "build", "--wheel", f"--config-setting=--plat-name={platform_tag}"]
    subprocess.run(args, check=True, cwd=script_dir, env=env)

    # Find the built wheel
    wheels = list(dist_dir.glob("*.whl"))
    if len(wheels) != 1:
        raise RuntimeError(f"Expected 1 wheel in {dist_dir}, but found {len(wheels)}")
    wheel_path = wheels[0]

    # Copy to output directory (skip if same location)
    output_dir.mkdir(parents=True, exist_ok=True)
    final_path = output_dir / wheel_path.name
    if wheel_path.resolve() != final_path.resolve():
        shutil.copy2(wheel_path, final_path)
    print(f"Created: {final_path}")

    return final_path


def main():
    parser = argparse.ArgumentParser(description="Build raymake wheels")
    parser.add_argument(
        "--platform",
        choices=list(PLATFORM_MAP.keys()) + ["all"],
        default="all",
        help="Platform to build (default: all)",
    )
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=Path(__file__).parent.parent.parent / "_release",
        help="Output directory for wheels (default: _release/)",
    )
    parser.add_argument(
        "--version",
        help="Version to package, overriding the checked-in VERSION file",
    )
    args = parser.parse_args()

    if args.platform == "all":
        platforms = list(PLATFORM_MAP.keys())
    else:
        platforms = [args.platform]

    script_dir = Path(__file__).parent
    if args.version:
        version = write_version_file(script_dir, args.version)
    else:
        version = read_version(script_dir)

    print(f"Packaging raymake {version}")
    print(f"Building wheels for: {', '.join(platforms)}")
    print(f"Output directory: {args.output_dir}")

    built_wheels = []
    for platform in platforms:
        wheel_path = build_wheel(platform, args.output_dir)
        built_wheels.append(wheel_path)

    print(f"\n{'=' * 60}")
    print("Build complete! Created wheels:")
    for wheel in built_wheels:
        print(f"  {wheel}")
    print(f"{'=' * 60}\n")


if __name__ == "__main__":
    main()
