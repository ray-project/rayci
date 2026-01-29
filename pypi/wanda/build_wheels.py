#!/usr/bin/env python3
# /// script
# requires-python = ">=3.9"
# dependencies = ["wheel"]
# ///
"""Build wanda wheels for all supported platforms.

Usage:
    # Build all platforms
    uv run pypi/wanda/build_wheels.py

    # Build specific platform
    uv run pypi/wanda/build_wheels.py --platform darwin-arm64
"""
import argparse
import os
import shutil
import subprocess
import sys
from pathlib import Path


def get_version_from_git() -> str:
    """Extract version from git tag (e.g., v0.27.0 -> 0.27.0)."""
    try:
        result = subprocess.run(
            ["git", "describe", "--tags", "--abbrev=0"],
            capture_output=True,
            text=True,
            check=True,
        )
        version = result.stdout.strip().lstrip("v")
        return version
    except subprocess.CalledProcessError:
        return "0.0.0"


def write_version_file(script_dir: Path) -> str:
    """Generate VERSION file from git tag."""
    version = get_version_from_git()
    version_file = script_dir / "VERSION"
    version_file.write_text(f'__version__ = "{version}"\n')
    print(f"Generated VERSION file: {version}")
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
        "platform": "manylinux_2_17_x86_64",
        "platform_expanded": "manylinux_2_17_x86_64.musllinux_1_1_x86_64",
    },
    "linux-arm64": {
        "goos": "linux",
        "goarch": "arm64",
        "platform": "manylinux_2_17_aarch64",
        "platform_expanded": "manylinux_2_17_aarch64.musllinux_1_1_aarch64",
    },
    "windows-amd64": {
        "goos": "windows",
        "goarch": "amd64",
        "platform": "win_amd64",
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
    platform_expanded = config.get("platform_expanded")

    print(f"\n{'='*60}")
    print(f"Building wheel for {platform_key}")
    print(f"  GOOS={goos} GOARCH={goarch}")
    print(f"  Platform tag: {platform_tag}")
    print(f"{'='*60}\n")

    # Set environment for cross-compilation
    env = os.environ.copy()
    env["GOOS"] = goos
    env["GOARCH"] = goarch
    env["CGO_ENABLED"] = "0"

    # Get the pypi/wanda directory
    script_dir = Path(__file__).parent
    dist_dir = script_dir / "dist"

    # Clean dist directory for this build
    if dist_dir.exists():
        shutil.rmtree(dist_dir)

    # Build the wheel
    args = [
        "uv",
        "build",
        "--wheel",
        f"--config-setting=--plat-name={platform_tag}",
    ]
    subprocess.run(args, check=True, cwd=script_dir, env=env)

    # Find the built wheel
    wheels = list(dist_dir.glob("*.whl"))
    if not wheels:
        raise RuntimeError(f"No wheel found in {dist_dir}")
    wheel_path = wheels[0]

    # For Linux, expand the platform tag to include musllinux
    if platform_expanded:
        print(f"Expanding platform tag to: {platform_expanded}")
        expand_args = [
            sys.executable,
            "-m",
            "wheel",
            "tags",
            "--remove",
            "--platform-tag",
            platform_expanded,
            str(wheel_path),
        ]
        subprocess.run(expand_args, check=True)
        # Find the renamed wheel
        wheels = list(dist_dir.glob("*.whl"))
        wheel_path = wheels[0]

    # Copy to output directory
    output_dir.mkdir(parents=True, exist_ok=True)
    final_path = output_dir / wheel_path.name
    shutil.copy2(wheel_path, final_path)
    print(f"Created: {final_path}")

    return final_path


def main():
    parser = argparse.ArgumentParser(description="Build wanda wheels")
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
    args = parser.parse_args()

    if args.platform == "all":
        platforms = list(PLATFORM_MAP.keys())
    else:
        platforms = [args.platform]

    # Generate VERSION file from git tag
    script_dir = Path(__file__).parent
    write_version_file(script_dir)

    print(f"Building wheels for: {', '.join(platforms)}")
    print(f"Output directory: {args.output_dir}")

    built_wheels = []
    for platform in platforms:
        wheel_path = build_wheel(platform, args.output_dir)
        built_wheels.append(wheel_path)

    print(f"\n{'='*60}")
    print("Build complete! Created wheels:")
    for wheel in built_wheels:
        print(f"  {wheel}")
    print(f"{'='*60}\n")


if __name__ == "__main__":
    main()
