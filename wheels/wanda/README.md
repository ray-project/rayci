# wanda-bin

Container image builder that uses a container registry as a content-addressed
build cache. Used for projects within https://github.com/ray-project

## Why Wanda?

- **Content-addressed caching** - Computes a digest from source files, base
  image digests, and build args, skipping builds entirely on cache hit.
- **Declarative spec files** - One `.wanda.yaml` declares all inputs. Share
  Dockerfiles across specs with different `build_args`. Supports `$VAR`
  expansion and `-env_file`.
- **Explicit build context** - Only files in `srcs` are sent to Docker.
  Dependencies automatically discovered and built in topological order.

## Installation

```bash
uv tool install wanda-bin

# Or with pip
pip install wanda-bin
```

## Usage

```bash
# Build from a spec file
wanda spec.wanda.yaml
```

## Distribution

Distributed as a pre-compiled Go binary via the wheel `scripts` data directory
([PEP 427]). The install location is determined by Python's [sysconfig]:

- `~/.local/bin/wanda` (Linux/macOS with `uv tool install`)
- `{venv}/bin/wanda` (virtual environment)
- `C:\Users\{user}\AppData\Local\Programs\Python\Scripts\wanda.exe` (Windows)

## Links

- [RayCI Repository]
- [Wanda Source]

[PEP 427]: https://peps.python.org/pep-0427/
[sysconfig]: https://docs.python.org/3/library/sysconfig.html#installation-paths
[RayCI Repository]: https://github.com/ray-project/rayci
[Wanda Source]: https://github.com/ray-project/rayci/tree/main/wanda
