# Code Reviewer Memory - rayci

## Codebase Overview
- Go monorepo for Ray CI/CD tooling on Buildkite
- Key packages: `raycicmd/` (pipeline gen), `wanda/` (container build), `rayapp/` (template builds/tests), `reefd/` (EC2 service), `goqualgate/` (quality gates)

## rayapp Package Patterns
- `AnyscaleCLI` struct wraps shell calls to `anyscale` binary via `exec.Command`
- `AnyscaleAPI` struct wraps direct HTTP API calls (uses `ANYSCALE_HOST` and `ANYSCALE_CLI_TOKEN` env vars)
- Tests use `writeFakeAnyscale()` to create mock shell scripts + `httptest.Server` for API mocks
- Tests mutate `os.Setenv`/`os.Unsetenv` directly -- not safe for `t.Parallel()`
- Error handling follows `fmt.Errorf("context: %w", err)` pattern consistently
- `runAnyscaleCLI` captures output via `tailWriter` (bounded 1MB buffer)

## Testing Patterns (PR3 branch)
- `fakeAnyscale` struct in `anyscale_cli_fake_test.go` - dispatches subcommands, supports override hooks
- `checkArgs`, `findPositionalArgs`, `hasPair` test helpers in `anyscale_cli_test_helpers_test.go`
- `setRunFunc` on `AnyscaleCLI` allows injecting test doubles without shell scripts
- `writeFakeAnyscale` (older pattern) creates actual shell script files for integration-style tests

## Convention Notes
- `NewAnyscaleCLI` is exported (capitalized) -- all uses are within same package
- `CopyFile` in `util.go` is exported but only used within package
- `fmt.Printf` used in production code for progress logging (no structured logging)

## Branch: elliot-barn-launch-template-into-workspace
- Adds `launchTemplateInWorkspace` to `anyscaleAPI` (POST /from_template)
- Adds `templateWorkspaceLauncher` implementing `workspaceLauncher` interface
- Adds `Probe()` flow: launch template via API -> wait -> run testCmd -> cleanup
- Adds `GetDefaultProject` CLI wrapper (new file `anyscale_cli_project.go`)
- `build_rayapp.sh` still modified for local dev (hardcoded path, missing exec)

## Common Issues Seen
- Debug `fmt.Println` statements were cleaned up (previously in anyscale_cli.go:37, anyscale_api.go:90-91)
- Unsafe type assertions on API response maps (`result["name"].(string)`) - now partially fixed with ok checks in template_test_runner.go:119-124
- `build_rayapp.sh` still modified for local dev (hardcoded path `/home/ubuntu/.local/bin/rayapp`, missing exec)
- `http.Client{}` created without Timeout in `newAnyscaleAPI` -- can hang indefinitely
- `WorkspaceTestConfig` has dead fields: configFile, imageURI, rayVersion (never read)
- Inconsistent capitalization in error messages: "get default Project" vs "get default project"
- `launchTemplateInWorkspace` lacks input validation (unlike `deleteWorkspaceByID` which validates ID)
- Branching on `c.template != nil` to select setup strategy is fragile implicit dispatch
