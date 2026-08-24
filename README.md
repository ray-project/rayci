This repository contains scripts and progresm for Ray CI/CD on buildkite.

- `rayci`: program that reads test definition files and generates buildkite
  pipeline definitions
- `wanda`: program that builds container images using a container registry as
  a content-addressed build cache

This repository also contains scripts that are used on legacy CI/CD pipelines
and KubeRay pipelines, in the `ray_ci` directory and `ecosystem_ci` directory.

## Tag Rules and Test Selection

`rayci` decides which pipeline steps to run on a pull request by mapping the
PR's changed files to a set of *tags*, then selecting the steps whose
declarations carry those tags. The mapping is driven by the `*.rules.txt` files
in the Buildkite config directories. The evaluator lives in
`raycicmd/tag_rule*.go`, with `RunTagAnalysis` (`raycicmd/tag_rule_run.go`) as
the entry point.

### The algorithm

Tag selection runs only for pull-request builds. On the master branch, a
`releases/*` branch, any non-PR build, or when `RAYCI_RUN_ALL_TESTS=1` is set,
`RunTagAnalysis` short-circuits to `["*"]` — every tag, the full pipeline. This
is why a merge to master runs everything regardless of what changed.

For a PR build, given the changed-file set:

1. Each `*.rules.txt` file loads into its **own** `TagRuleSet`. Multiple rules
   files are evaluated independently and never concatenated. Ray, for example,
   keeps `test.rules.txt` and `always.rules.txt` as separate rulesets.
1. For each changed file, each ruleset returns the tags of its **first matching
   rule** — first-match-wins, in file order (`TagRuleSet.MatchTags`). A rule
   with no `@` tags is a skip rule: it matches, ends that ruleset's search for
   the file, and contributes no tags.
1. The per-file results union across every ruleset and every changed file into
   one tag set (`tagsForChangedFiles`).

A file that matches no rule in any ruleset is logged as `unhandled file (no
matching rule)` and contributes nothing, so a rules file meant to cover
everything ends with a `*` catch-all.

### Rule file grammar

A `*.rules.txt` file is a sequence of tag declarations followed by rules:

- `! tag ...` declares one or more tags. All declarations must precede the first
  rule, and every tag a rule emits must be declared or loading the config errors
  (`ValidateRules`).
- A token ending in `/` is a directory-prefix match: the path must equal the
  directory or sit under it.
- A token with no glob character is an exact file-path match.
- A token containing `*` or `?` is a glob, compiled to an anchored regex where
  `*` becomes `.*` and `?` becomes `.`, both crossing `/`. Every other regex
  metacharacter is escaped to a literal (`globToRegexp`), so a bracket
  expression such as `[abc]` matches those literal characters, not a character
  class.
- `@ tag ...` sets the tags the current rule emits. A rule with no `@` line is a
  skip rule.
- `;` ends a rule.

### The `test-rules` conformance check

`rayci test-rules` validates that the rules compute the tags a repo expects. It
discovers each `*.rules.txt` file and runs its companion `*.rules.test.txt`
fixture when one exists. The companion name is the rules filename with `.txt`
replaced by `.test.txt` (`go.rules.txt` is tested by `go.rules.test.txt`); a
rules file with no companion is not checked. Auto-discovery is overridable with
`test_rules_files` in config or the `RAYCI_TEST_RULE_FILES` environment
variable.

Each fixture line is `path: tag1 tag2 ...`, asserting the tag set the evaluator
computes for that path. The comparison is **exact set equality**, not a subset
check, so a line must enumerate *every* tag the path emits — asserting a single
tag is a positive claim that no other tag fires. `test-rules` runs the same
`tagsForChangedFiles` matcher that live per-PR selection uses, so a green run
validates the exact algorithm that selects steps.

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
