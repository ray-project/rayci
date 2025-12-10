package raycicmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestRunTagAnalysis_ConfigFileNotExist(t *testing.T) {
	envs := newEnvsMap(map[string]string{
		"BUILDKITE":              "true",
		"BUILDKITE_PULL_REQUEST": "123",
	})

	tags, err := RunTagAnalysis(
		[]string{"/nonexistent/path/to/config.txt"},
		envs,
		nil,
	)
	if err != nil {
		t.Fatalf("RunTagAnalysis() unexpected error: %v", err)
	}

	want := []string{"*"}
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("RunTagAnalysis() = %v, want %v", tags, want)
	}
}

const testsYAML = `
ci/pipeline/test_conditional_testing.py: lint tools
python/ray/data/__init__.py: lint data ml train
doc/index.md: lint

python/ray/air/__init__.py: lint ml train tune data linux_wheels
python/ray/llm/llm.py: lint llm
python/ray/workflow/workflow.py: lint workflow
python/ray/tune/tune.py: lint ml train tune linux_wheels
python/ray/train/train.py: lint ml train linux_wheels
python/ray/util/dask/dask.py: lint python dask
.buildkite/ml.rayci.yml: lint ml train tune
rllib/rllib.py: lint rllib rllib_gpu rllib_directly

python/ray/serve/serve.py: lint serve linux_wheels java
python/ray/dashboard/dashboard.py: lint dashboard linux_wheels python
python/core.py:
    - lint ml tune train data
    - python dashboard linux_wheels macos_wheels java
python/setup.py:
    - lint ml tune train serve workflow data
    - python dashboard linux_wheels macos_wheels java python_dependencies
python/requirements/test-requirements.txt:
    - lint ml tune train serve workflow data
    - python dashboard linux_wheels macos_wheels java python_dependencies
python/_raylet.pyx:
    - lint ml tune train data
    - python dashboard linux_wheels macos_wheels java
python/ray/dag/dag.py:
    - lint python cgraphs_direct_transport
python/ray/experimental/gpu_object_manager/gpu_object_manager.py:
    - lint python cgraphs_direct_transport

.buildkite/core.rayci.yml: lint python core_cpp
java/ray.java: lint java
.buildkite/others.rayci.yml: lint java
cpp/ray.cc: lint cpp
docker/Dockerfile.ray: lint docker linux_wheels

.readthedocs.yaml: lint doc
doc/code.py: lint doc
doc/example.ipynb: lint doc
doc/tutorial.rst: lint doc
.vale.ini: lint doc
.vale/styles/config/vocabularies/Core/accept.txt: lint doc

ci/docker/doctest.build.Dockerfile: lint
release/requirements.txt: lint release_tests
release/requirements_buildkite.txt: lint tools
release/release_tests.yaml: lint tools
ci/lint/lint.sh: lint tools
.buildkite/lint.rayci.yml: lint tools
.buildkite/macos.rayci.yml: lint macos_wheels
ci/ray_ci/tester.py: lint tools
.buildkite/base.rayci.yml: lint docker linux_wheels tools
ci/ci.sh: lint tools

src/ray.cpp:
    - lint core_cpp cpp java python
    - linux_wheels macos_wheels dashboard release_tests

.github/CODEOWNERS: lint
README.rst: lint
BUILD.bazel:
    - lint ml tune train data serve core_cpp cpp java
    - python doc linux_wheels macos_wheels dashboard tools
    - release_tests
`

var testRulesSnapshot = filepath.Join(
	"data",
	"62231dd4ba8e784da8800b248ad7616b8db92de7.txt",
)

func runCommandFromDirectory(cmd *exec.Cmd, dir string) ([]byte, error) {
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run command: %w", err)
	}
	return output, nil
}

type FileToTags struct {
	File string
	Tags []string
}

// Set of tags.
type TestTagSet map[string]struct{}

// Custom Unmarshal for either:
//
//	key: "lint tools"
//
// or
//
//	key:
//	  - "lint ml train"
//	  - "python dashboard"
func (s *TestTagSet) UnmarshalYAML(value *yaml.Node) error {
	if *s == nil {
		*s = make(TestTagSet)
	}
	addLine := func(s2 string) {
		for _, f := range strings.Fields(s2) {
			(*s)[f] = struct{}{}
		}
	}

	switch value.Kind {
	case yaml.ScalarNode:
		addLine(value.Value)
	case yaml.SequenceNode:
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf(
					"expected scalar in sequence, got kind %d",
					item.Kind,
				)
			}
			addLine(item.Value)
		}
	default:
		return fmt.Errorf("unsupported YAML node kind %d", value.Kind)
	}

	return nil
}

// Top-level: path -> tag set
type TestTagMap map[string]TestTagSet

// Mimicking test_conditional_testing_pull_request from
// https://github.com/ray-project/ray/blob/c963d646f0197947429b374cb06f831b47aab5dd/ci/pipeline/test_conditional_testing.py#L87
func TestWithTestRulesSnapshot(t *testing.T) {
	raw := TestTagMap{}

	if err := yaml.Unmarshal([]byte(testsYAML), &raw); err != nil {
		t.Fatal(err)
	}

	testCases := make([]FileToTags, 0, len(raw))
	for file, tagSet := range raw {
		tags := make([]string, 0, len(tagSet))
		for tag := range tagSet {
			tags = append(tags, tag)
		}
		// optional: sort for deterministic order in tests
		sort.Strings(tags)

		testCases = append(testCases, FileToTags{
			File: file,
			Tags: tags,
		})
	}

	origin := t.TempDir()
	workDir := t.TempDir()

	// Initialize bare repo
	runGitCommand(t, origin, "init", "--bare")

	// Initialize working repo
	runGitCommand(t, workDir, "init")
	runGitCommand(t, workDir, "config", "user.email", "rayci@ray.io")
	runGitCommand(t, workDir, "config", "user.name", "Ray CI Test")
	runGitCommand(t, workDir, "remote", "add", "origin", origin)

	// Write and commit README.md
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# README\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitCommand(t, workDir, "add", "README.md")
	runGitCommand(t, workDir, "commit", "-m", "initial commit")
	runGitCommand(t, workDir, "push", "origin", "master")

	for _, tc := range testCases {
		runGitCommand(t, workDir, "checkout", "-B", "pr01", "master")

		tcFilePath := filepath.Join(workDir, tc.File)

		// Make all directories needed for the file
		dirname := filepath.Dir(tcFilePath)
		if err := os.MkdirAll(dirname, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dirname, err)
		}

		if err := os.WriteFile(tcFilePath, []byte("...\n"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		runGitCommand(t, workDir, "add", ".")
		runGitCommand(t, workDir, "commit", "-m", "add test files")
		output, err := runCommandFromDirectory(
			exec.Command("git", "show", "HEAD", "-q", "--format=%H"),
			workDir,
		)
		if err != nil {
			t.Fatalf("git show HEAD: %v", err)
		}
		commit := strings.TrimSpace(string(output))

		envs := newEnvsMap(map[string]string{
			"BUILDKITE":                          "true",
			"BUILDKITE_PULL_REQUEST_BASE_BRANCH": "master",
			"BUILDKITE_PULL_REQUEST":             "1",
			"BUILDKITE_COMMIT":                   commit,
		})

		tags, err := RunTagAnalysis(
			[]string{testRulesSnapshot},
			envs,
			&RealGitClient{WorkDir: workDir},
		)
		if err != nil {
			t.Fatalf("RunTagAnalysis: %v", err)
		}

		want := append(tc.Tags, "always")

		sort.Strings(want)
		if !reflect.DeepEqual(tags, want) {
			t.Errorf("RunTagAnalysis(): got %v, want %v", tags, want)
		}
	}
}
