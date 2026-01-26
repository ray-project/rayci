# AGENTS.md

Guidance for AI agents working in this repository.

## What this repo is

CI/CD tooling for the Ray project on Buildkite:

- **rayci**: Generates Buildkite pipeline definitions from `.buildkite/*.rayci.yaml`
- **wanda**: Builds container images using a container registry as a content-addressed build cache
- **rayapp**: Builds Ray application templates (zip artifacts)
- **raycirun**: Library for triggering Buildkite builds programmatically
- **reefd**: EC2/database service with reaper functionality
- **goqualgate**: Go quality gates for CI (test coverage checks)

If you are unsure where logic lives:
- pipeline generation + rules + selection: `raycicmd/`
- Buildkite "step conversion": likely in `raycicmd/` (strategy converters)
- container build + registry/cache: `wanda/`
- template zips: `rayapp/`
- build triggering: `raycirun/`
- Go quality gates: `goqualgate/`

---

## First steps for any change

1. **Understand the command/package boundary** you’re touching (CLI vs library).
2. Make the smallest change that preserves existing behavior.
3. Run targeted tests locally, then `go test ./...`, then `go fmt ./...`.
4. Keep exported APIs narrow and stable.

---

## Build Commands

```bash
# Build rayci
go build .

# Build wanda
go build ./wanda/wanda

# Build rayapp
go build ./rayapp/rayapp

# Build goqualgate
go build ./goqualgate/goqualgate

# Run all tests
go test ./...

# Run tests for a specific package
go test ./raycicmd
go test ./wanda

# Run a specific test
go test ./raycicmd -run TestConverterBasic

# Format code
go fmt ./...

# Build release binaries (creates _release/ directory)
bash release.sh
```

## Running the Tools

```bash
# rayci: generate pipeline YAML
./rayci -repo . -output pipeline.yaml
./rayci -repo . -output -           # output to stdout
./rayci -upload                      # upload directly to Buildkite

# wanda: build container image (local mode)
./wanda spec.wanda.yaml

# wanda: build in RayCI mode (uses RAYCI_* env vars)
./wanda -rayci

# goqualgate: run all quality gates with defaults
./goqualgate all

# goqualgate: run coverage checks (default minimum 80%)
./goqualgate coverage
./goqualgate coverage -min-coverage-pct=90

# goqualgate: check file lengths (default max 300 lines)
./goqualgate filelength
./goqualgate filelength -max-lines=400
```

## Architecture

### RayCI Pipeline Generation Flow
1. Scans `.buildkite/*.rayci.yaml` for step definitions
2. Detects changed files via git diff (for PRs)
3. Applies tag rules from `.buildkite/*.rules.txt` to determine which tests run
4. Filters steps based on tags and selections
5. Outputs Buildkite pipeline YAML

### Wanda Container Build Flow
1. Parses `.wanda.yaml` specification (name, dockerfile, srcs, froms, build_args)
2. Computes content-addressed digest from build inputs
3. Checks cache in registry - skips build on cache hit
4. Builds with Docker and pushes to work repository

### Tag Rule System
Tag rules (`.buildkite/*.rules.txt`) map file paths to test tags:
- Lines declare tags with `! tag_name`
- Patterns match directories (`dir/`), files, or globs (`*.py`)
- `@ tag1 tag2` emits tags for matched files; rules without `@` are skipping rules
- `;` separates rules
- Test files (`*.rules.test.txt`) validate rules with format `path: expected_tag1 expected_tag2`
- Used for conditional testing based on changed files

### Step Converter Pattern
Pipeline steps use a strategy pattern with multiple converters (`waitConverter`, `blockConverter`, `triggerConverter`, `wandaConverter`, `commandConverter`). Converters implement `match()` and `convert()` methods; first match wins with `commandConverter` as fallback.

### Dynamic YAML Handling
Buildkite steps are represented as `map[string]any` to handle dynamic YAML structures. Helper functions extract typed values: `stringInMap()`, `boolInMap()`, `intInMap()`, `toStringList()`.

## Code Style

### Comments

- Avoid obvious comments that describe what the code literally does
- Keep comments for: exported functions (godoc), non-obvious behavior, algorithm explanations
- Prefer self-documenting code over comments
- Remove comments like `// Parse the spec file` before `ParseSpecFile()` calls

### Go Package Interfaces

Keep package interfaces narrow. Prefer unexported functions and types unless they need to be accessed from outside the package. For subcommands or internal logic, use unexported functions with the same signature as the main entry point, and separate testable inner functions if needed.

```go
// Good: narrow interface
func Main(args []string, envs Envs) error {
    if args[1] == "test-rules" {
        return subcmdTestRules(args[2:], envs)  // unexported
    }
    // ...
}

func subcmdTestRules(args []string, envs Envs) error {
    return execTestRules(args, envs, os.Stdout)  // unexported, testable
}

// Bad: unnecessarily wide interface
func TestRulesMain(args []string, envs Envs, stdout io.Writer) error {  // exported
    // ...
}
```

### Go Structs

Use pointers to structs by default (e.g., `[]*MyStruct` instead of `[]MyStruct`) when performance or memory footprint is not a concern. This avoids needing to revisit the decision when adding more fields to the struct later.

### Error Handling

Wrap errors with context using `%w` at each level. Errors bubble up to Main without inline logging:

```go
if err != nil {
    return fmt.Errorf("parse config file: %w", err)
}
```

### Constructor Naming

Use unexported `newTypeName` constructors (not `NewTypeName`):

```go
func newBuildInput(spec *Spec, dir string) (*buildInput, error) { ... }
```

### Sets

Emulate sets using `map[string]struct{}`:

```go
seen := make(map[string]struct{})
seen[key] = struct{}{}
if _, ok := seen[key]; ok { ... }
```

### Testability via Interfaces

Environment variables are accessed through the `Envs` interface, allowing test stubs:

```go
type Envs interface {
    Getenv(string) string
}

// Production: osEnvs wraps os.Getenv
// Tests: newEnvsMap(map[string]string{...})
```

### Tests

- Format test errors using "got, want" ordering:
  ```go
  if got != want {
      t.Errorf("FunctionName() = %v, want %v", got, want)
  }
  ```

- For multiline test strings, prefer `strings.Join()` over backtick literals:
  ```go
  // Preferred
  input := strings.Join([]string{"! mytag", "python/", "@ mytag", ";"}, "\n")

  // Avoid
  input := `! mytag
  python/
  @ mytag
  ;`
  ```
