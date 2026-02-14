This repository contains scripts and progresm for Ray CI/CD on buildkite.

- `rayci`: program that reads test definition files and generates buildkite
  pipeline definitions
- `wanda`: program that builds container images using a container registry as
  a content-addressed build cache

This repository also contains scripts that are used on legacy CI/CD pipelines
and KubeRay pipelines, in the `ray_ci` directory and `ecosystem_ci` directory.

## Development Setup

### Pre-commit Hooks

This project uses pre-commit hooks for automated code quality enforcement.

**Install pre-commit:**
```bash
pip install pre-commit
# or
brew install pre-commit
```

**Install the git hooks:**
```bash
pre-commit install --install-hooks
```

This will automatically install all required tools (including `golines` for line-length enforcement) in pre-commit's isolated environment.

The hooks will run automatically on:
- **Commit** (fast checks): `go fmt`, `golines` (100-char line limit)
- **Push** (comprehensive checks): `go vet ./...`, `goqualgate all`

**To bypass hooks in emergencies:**
```bash
git commit --no-verify
git push --no-verify
```
