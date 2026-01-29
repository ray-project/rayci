"""PDM build hook to compile the wanda Go binary during wheel build."""

import os
import shutil
import subprocess
import sys
from pathlib import Path

try:
    import tomllib
except ImportError:
    import tomli as tomllib


def _get_config():
    pyproject = Path(__file__).parent / "pyproject.toml"
    with open(pyproject, "rb") as f:
        return tomllib.load(f)["tool"]["wanda"]


CONFIG = _get_config()
BIN_NAME = CONFIG["bin_name"]


def is_windows():
    if "GOOS" in os.environ:
        return os.environ["GOOS"] == "windows"
    return sys.platform == "win32"


def build(output: str) -> None:
    """Compile the wanda Go binary."""
    go = shutil.which("go")
    if go is None:
        raise RuntimeError("golang is required and 'go' should be in $PATH")

    os.environ.setdefault("CGO_ENABLED", "0")

    repo_root = Path(__file__).parent.parent.parent
    wanda_pkg = repo_root / "wanda" / "wanda"

    if not wanda_pkg.exists():
        raise RuntimeError(f"wanda source not found at {wanda_pkg}")

    args = [
        go,
        "build",
        "-o",
        output,
        "-trimpath",
        "-ldflags",
        "-s -w",
        str(wanda_pkg),
    ]

    print(f"Building wanda: {' '.join(args)}")
    subprocess.run(args, check=True, cwd=repo_root)
    Path(output).chmod(0o755)
    print(f"Built wanda binary: {output}")


def pdm_build_initialize(context):
    """Compile the Go binary before wheel packaging."""
    setting = {"--python-tag": "py3", "--py-limited-api": "none"}
    context.builder.config_settings = {**setting, **context.builder.config_settings}
    context.ensure_build_dir()

    output_path = Path(context.build_dir, "bin", BIN_NAME)
    if is_windows():
        output_path = output_path.with_suffix(".exe")
    output_path.parent.mkdir(parents=True, exist_ok=True)

    build(str(output_path))
