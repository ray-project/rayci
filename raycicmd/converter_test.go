package raycicmd

import (
	"testing"

	"encoding/json"
	"fmt"
	"reflect"
)

func TestParseStepEnvs(t *testing.T) {
	for _, test := range []struct {
		in      any
		want    []*envEntry
		wantErr bool
	}{
		{in: nil, wantErr: true},
		{in: "string", wantErr: true},
		{in: []string{"A=B"}, wantErr: true},
		{in: map[string]string{"A": "B"}, wantErr: true},
		{
			in: map[string]any{"PY_VERSION": "py36", "A": "B"},
			want: []*envEntry{
				{k: "A", v: "B"},
				{k: "PY_VERSION", v: "py36"},
			},
		},
	} {
		got, err := parseStepEnvs(test.in)
		if test.wantErr {
			if err == nil {
				t.Errorf("parseStepEnvs(%+v): want error, got nil", test.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseStepEnvs(%+v) got error: %v", test.in, err)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf(
				"parseStepEnvs(%+v): got %+v, want %+v",
				test.in, got, test.want,
			)
		}
	}
}

func findPlugin(plugins []any, name string) (map[string]any, bool) {
	for _, p := range plugins {
		if m, ok := p.(map[string]any); ok {
			v, ok := m[name]
			if ok {
				return v.(map[string]any), true
			}
		}
	}
	return nil, false
}

func findDockerPlugin(plugins []any) (map[string]any, bool) {
	return findPlugin(plugins, dockerPlugin)
}

func findAWSAssumeRolePlugin(plugins []any) (map[string]any, bool) {
	return findPlugin(plugins, awsAssumeRolePlugin)
}

func findMacosSandboxPlugin(plugins []any) bool {
	for _, p := range plugins {
		if m, ok := p.(map[string]any)[macosSandboxPlugin]; ok {
			if f, ok := m.(map[string]string)["deny-file-read"]; ok && f == macosDenyFileRead {
				return true
			}
		}
	}

	return false
}

func findInSlice(s []string, v string) bool {
	for _, e := range s {
		if e == v {
			return true
		}
	}
	return false
}

func TestConvertPipelineStep_concurrency_group(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		buildID:        buildID,
		launcherBranch: "beta",
		gitCommit:      "abcdefg1234567890",
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		CIWorkRepo:      "fakeecr",
		RunnerQueues: map[string]string{
			"default": "fakerunner",
		},
		ConcurrencyGroupPrefixes: []string{"not_group"},
	}, info)

	step := map[string]any{
		"label":             "say hello",
		"key":               "key",
		"command":           "echo hello",
		"concurrency":       2,
		"concurrency_group": "group",
	}
	if _, err := c.convertStep("fakeid", step); err == nil {
		t.Errorf("TestConvertPipelineStep_concurrency_group %+v: step concurrent group should not be allowed", step)
	}
}

func TestConvertPipelineStep(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		buildID:        buildID,
		launcherBranch: "beta",
		gitCommit:      "abcdefg1234567890",
	}

	const fakeStepID = "fakeid"

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		CIWorkRepo:      "fakeecr",

		RunnerQueues: map[string]string{
			"default": "fakerunner",
			"windows": "fakewinrunner",
			"macos":   "fakemacrunner",
			"broken":  skipQueue,
		},

		Env: map[string]string{
			"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
		},

		BuildEnvKeys:   []string{"RAYCI_SCHEDULE"},
		HookEnvKeys:    []string{"RAYCI_CHECKOUT_DIR"},
		MaxParallelism: 5,
	}, info)

	const artifactDest = "s3://artifacts_bucket/abcdefg1234567890/abc123"

	for _, test := range []struct {
		in  map[string]any
		out map[string]any // buildkite pipeline step

		dockerPluginOut map[string]any // extra fields expected in docker plugin
	}{{
		in: map[string]any{
			"commands": []string{"echo 1", "echo 2"},
		},
		out: map[string]any{
			"label":              "[fakeid]",
			"commands":           []string{"echo 1", "echo 2"},
			"agents":             newBkAgents("fakerunner"),
			"timeout_in_minutes": defaultTimeoutInMinutes,
			"artifact_paths":     defaultArtifactPaths,
			"retry":              defaultRayRetry,

			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",
				"RAYCI_STEP_ID":             fakeStepID,

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
		},
	}, {
		in: map[string]any{
			"commands":           []string{"echo 1", "echo 2"},
			"timeout_in_minutes": 10,
		},
		out: map[string]any{
			"label":              "[fakeid]",
			"commands":           []string{"echo 1", "echo 2"},
			"agents":             newBkAgents("fakerunner"),
			"timeout_in_minutes": 10,
			"artifact_paths":     defaultArtifactPaths,
			"retry":              defaultRayRetry,

			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",
				"RAYCI_STEP_ID":             fakeStepID,

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
		},
	}, {
		in: map[string]any{
			"name":                     "myname",
			"commands":                 []string{"echo 1", "echo 2"},
			"docker_publish_tcp_ports": "5555,5556",
		},
		out: map[string]any{
			"label":              "myname [fakeid]",
			"commands":           []string{"echo 1", "echo 2"},
			"agents":             newBkAgents("fakerunner"),
			"timeout_in_minutes": defaultTimeoutInMinutes,
			"artifact_paths":     defaultArtifactPaths,
			"retry":              defaultRayRetry,

			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",
				"RAYCI_STEP_ID":             fakeStepID,

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
		},
		dockerPluginOut: map[string]any{
			"publish": []string{
				"127.0.0.1:5555:5555/tcp", "127.0.0.1:5556:5556/tcp",
			},
		},
	}, {
		in: map[string]any{
			"commands":       []string{"echo 1", "echo 2"},
			"docker_network": "host",
		},
		out: map[string]any{
			"label":              "[fakeid]",
			"commands":           []string{"echo 1", "echo 2"},
			"agents":             newBkAgents("fakerunner"),
			"timeout_in_minutes": defaultTimeoutInMinutes,
			"artifact_paths":     defaultArtifactPaths,
			"retry":              defaultRayRetry,

			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",
				"RAYCI_STEP_ID":             fakeStepID,

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
		},
		dockerPluginOut: map[string]any{"network": "host"},
	}, {
		in: map[string]any{
			"commands":      []string{"echo 1"},
			"instance_type": "broken",
		},
		out: map[string]any{
			"label":              "[fakeid]",
			"commands":           []string{"echo 1"},
			"timeout_in_minutes": defaultTimeoutInMinutes,
			"artifact_paths":     defaultArtifactPaths,
			"retry":              defaultRayRetry,
			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",
				"RAYCI_STEP_ID":             fakeStepID,

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
			"skip": true,
		},
	}, {
		in: map[string]any{
			"label":                    "say hello",
			"key":                      "key",
			"command":                  "echo hello",
			"depends_on":               "dep",
			"allow_dependency_failure": true,
		},
		out: map[string]any{
			"label":                    "say hello [fakeid]",
			"key":                      "key",
			"command":                  "echo hello",
			"depends_on":               "dep",
			"allow_dependency_failure": true,

			"agents": newBkAgents("fakerunner"),

			"timeout_in_minutes": defaultTimeoutInMinutes,
			"artifact_paths":     defaultArtifactPaths,
			"retry":              defaultRayRetry,
			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",
				"RAYCI_STEP_ID":             fakeStepID,

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
		},
	}, {
		in: map[string]any{
			"label":             "say hello",
			"key":               "key",
			"command":           "echo hello",
			"depends_on":        "dep",
			"concurrency":       2,
			"concurrency_group": "group",
		},
		out: map[string]any{
			"label":             "say hello [fakeid]",
			"key":               "key",
			"command":           "echo hello",
			"depends_on":        "dep",
			"concurrency":       2,
			"concurrency_group": "group",

			"agents": newBkAgents("fakerunner"),

			"timeout_in_minutes": defaultTimeoutInMinutes,
			"artifact_paths":     defaultArtifactPaths,
			"retry":              defaultRayRetry,
			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",
				"RAYCI_STEP_ID":             fakeStepID,

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
		},
	}, {
		in: map[string]any{
			"name":       "forge",
			"label":      "my forge",
			"wanda":      "ci/forge.wanda.yaml",
			"depends_on": "ci-base",
			"env":        map[string]any{"PY_VERSION": "{{matrix}}"},
			"matrix":     []any{"py36", "py37"},
		},
		out: map[string]any{
			"label":    "my forge",
			"key":      "forge",
			"commands": wandaCommands("beta"),
			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",

				"RAYCI_WANDA_FILE": "ci/forge.wanda.yaml",
				"RAYCI_WANDA_NAME": "forge",

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,

				"PY_VERSION": "{{matrix}}",
			},
			"matrix":     []any{"py36", "py37"},
			"depends_on": "ci-base",
			"retry": map[string]any{
				"automatic": map[string]any{"limit": 1},
			},
			"timeout_in_minutes": 300,
		},
	}, {
		in: map[string]any{
			"label":         "windows job",
			"key":           "win",
			"command":       "echo windows",
			"job_env":       "WINDOWS",
			"instance_type": "windows",
		},
		out: map[string]any{
			"label":   "windows job [fakeid]",
			"key":     "win",
			"command": "echo windows",
			"agents":  newBkAgents("fakewinrunner"),

			"artifact_paths":     windowsArtifactPaths,
			"timeout_in_minutes": defaultTimeoutInMinutes,
			"retry":              defaultRayRetry,
			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",
				"RAYCI_STEP_ID":             fakeStepID,

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
		},
	}, {
		in: map[string]any{
			"label":         "windows job",
			"key":           "win",
			"command":       "echo windows",
			"job_env":       "WINDOWS",
			"instance_type": "windows",
		},
		out: map[string]any{
			"label":          "windows job [fakeid]",
			"key":            "win",
			"command":        "echo windows",
			"agents":         newBkAgents("fakewinrunner"),
			"artifact_paths": []string{"C:\\tmp\\artifacts\\**\\*"},

			"timeout_in_minutes": defaultTimeoutInMinutes,
			"retry":              defaultRayRetry,
			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",
				"RAYCI_STEP_ID":             fakeStepID,

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
		},
	}, {
		in: map[string]any{
			"label":         "mac job",
			"key":           "mac",
			"command":       "echo mac",
			"job_env":       "MACOS",
			"instance_type": "macos",
			"parallelism":   4,
		},
		out: map[string]any{
			"label":   "mac job [fakeid]",
			"key":     "mac",
			"command": "echo mac",
			"agents":  newBkAgents("fakemacrunner"),

			"timeout_in_minutes": defaultTimeoutInMinutes,
			"retry":              defaultRayRetry,
			"env": map[string]string{
				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,

				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_BRANCH":              "beta",
				"RAYCI_BUILD_ID":            "abc123",
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_STEP_ID":             fakeStepID,
			},
			"parallelism": 4,
		},
	}, {
		in: map[string]any{
			"block": "block", "tags": []string{"foo"},
			"if": "false", "depends_on": "dep",
		},
		out: map[string]any{
			"block": "block", "if": "false", "depends_on": "dep",
		},
	}, {
		in:  map[string]any{"wait": nil},
		out: map[string]any{"wait": nil},
	}, {
		in:  map[string]any{"wait": nil, "tags": []string{"foo"}},
		out: map[string]any{"wait": nil},
	}, {
		in: map[string]any{
			"wait": nil, "continue_on_failure": true,
			"depends_on": "dep", "if": "false",
		},
		out: map[string]any{
			"wait": nil, "continue_on_failure": true,
			"depends_on": "dep", "if": "false",
		},
	}, {
		in: map[string]any{
			"label":       "say hello",
			"command":     "echo hello",
			"parallelism": 200,
		},
		out: map[string]any{
			"agents":             newBkAgents("fakerunner"),
			"timeout_in_minutes": defaultTimeoutInMinutes,
			"artifact_paths":     defaultArtifactPaths,
			"retry":              defaultRayRetry,
			"env": map[string]string{
				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
				"BUILDKITE_BAZEL_CACHE_URL":             "https://bazel-build-cache",
				"RAYCI_BRANCH":                          "beta",
				"RAYCI_BUILD_ID":                        "abc123",
				"RAYCI_STEP_ID":                         "fakeid",
				"RAYCI_TEMP":                            "s3://ci-temp/abc123/",
				"RAYCI_WORK_REPO":                       "fakeecr",
			},
			"label":       "say hello [fakeid]",
			"command":     "echo hello",
			"parallelism": 5,
		},
	}} {
		got, err := c.convertStep("fakeid", test.in)
		if err != nil {
			t.Errorf("convertPipelineStep %+v: %v", test.in, err)
			continue
		}
		if got == nil {
			if test.out != nil {
				t.Errorf(
					"convertPipelineStep %+v: got:\n %s\nwant:\n %s",
					test.in, got, test.out,
				)
			}
			continue
		}

		_, isBlock := got["block"]
		_, isWait := got["wait"]
		_, isWanda := test.in["wanda"]

		plugins, ok := got["plugins"]
		if ok {
			// Check non plugins only.
			delete(got, "plugins")
		}

		if !reflect.DeepEqual(got, test.out) {
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal got: +%v: %s", got, err)
			}
			wantJSON, err := json.MarshalIndent(test.out, "", "  ")
			if err != nil {
				t.Fatalf("marshal want: +%v: %s", test.out, err)
			}

			t.Errorf(
				"convertPipelineStep %+v: got:\n %s\nwant:\n %s",
				test.in, gotJSON, wantJSON,
			)
		}

		if isWait || isBlock || isWanda {
			continue
		}

		jobEnv, ok := test.in["job_env"]
		if ok && jobEnv == macosJobEnv {
			if !findMacosSandboxPlugin(plugins.([]any)) {
				t.Errorf("convertPipelineStep %+v: no macos sandbox plugin", test.in)
			}
			continue
		}

		dockerPlugin, ok := findDockerPlugin(plugins.([]any))
		if !ok {
			t.Errorf("convertPipelineStep %+v: no docker plugin", test.in)
			continue
		}

		image, ok := stringInMap(dockerPlugin, "image")
		if !ok {
			t.Errorf("convertPipelineStep %+v: no docker image", test.in)
		}
		if test.in["job_env"] == windowsJobEnv {
			if want := windowsBuildEnvImage; image != want {
				t.Errorf(
					"convertPipelineStep %+v: got docker image %q, want %q",
					test.in, image, want,
				)
			}
		} else {
			if want := fmt.Sprintf("fakeecr:%s-forge", buildID); image != want {
				t.Errorf(
					"convertPipelineStep %+v: got docker image %q, want %q",
					test.in, image, want,
				)
			}
		}

		envs := dockerPlugin["environment"].([]string)

		for _, env := range []string{
			"RAYCI_BUILD_ID",
			"RAYCI_TEMP",
			"RAYCI_WORK_REPO",
			"BUILDKITE_BAZEL_CACHE_URL",
			"RAYCI_SCHEDULE",
			"RAYCI_CHECKOUT_DIR",
			"RAYCI_STEP_ID",
		} {
			if !findInSlice(envs, env) {
				t.Errorf("convertPipelineStep %+v: no %q", test.in, env)
			}
		}

		for k, v := range test.dockerPluginOut {
			if !reflect.DeepEqual(dockerPlugin[k], v) {
				t.Errorf(
					"convertPipelineStep %+v: "+
						"got %+v for docker plugin %q, want %+v",
					test.in, dockerPlugin[k], k, v,
				)
			}
		}
	}
}

func convertSingleGroup(c *converter, g *pipelineGroup, filter *stepFilter) (
	*bkPipelineGroup, error,
) {
	result, err := c.convertGroups([]*pipelineGroup{g}, filter)
	if err != nil {
		return nil, err
	}
	if len(result) != 1 {
		return nil, fmt.Errorf("got %d groups, want 1", len(result))
	}
	return result[0], nil
}

func TestConvertPipelineGroup_priority(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		buildID:        buildID,
		launcherBranch: "beta",
		gitCommit:      "abcdefg1234567890",
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		CIWorkRepo:      "fakeecr",

		RunnerQueues: map[string]string{"default": "fakerunner"},

		Env: map[string]string{
			"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
		},

		RunnerPriority: 1,
	}, info)

	g := &pipelineGroup{
		Group: "fancy",
		Steps: []map[string]any{
			{"commands": []string{"high priority"}, "priority": 10},
			{"wait": nil},
			{"commands": []string{"default priority"}},
		},
	}
	filter := &stepFilter{runAll: true}
	bk, err := convertSingleGroup(c, g, filter)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	steps := bk.Steps
	if p := (steps[0].(map[string]any))["priority"]; p != 10 {
		t.Errorf("high priority step: got priority %v, want 10", p)
	}
	if p := (steps[2].(map[string]any))["priority"]; p != 1 {
		t.Errorf("low priority step: got priority %v, want 1", p)
	}
}

func TestConvertPipelineGroup_defaultJobEnv(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		buildID:        buildID,
		launcherBranch: "beta",
		gitCommit:      "abcdefg1234567890",
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		CIWorkRepo:      "fakeecr",

		RunnerQueues: map[string]string{"default": "fakerunner"},

		Env: map[string]string{
			"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
		},
	}, info)

	g := &pipelineGroup{
		Group:         "fancy",
		DefaultJobEnv: "premerge",
		Steps: []map[string]any{
			{"commands": []string{"high priority"}, "priority": 10},
			{"wait": nil},
			{
				"commands": []string{"default priority"},
				"job_env":  "postmerge",
			},
		},
	}
	filter := &stepFilter{runAll: true}
	bk, err := convertSingleGroup(c, g, filter)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	premergeImage := fmt.Sprintf("fakeecr:%s-premerge", buildID)
	postmergeImage := fmt.Sprintf("fakeecr:%s-postmerge", buildID)
	steps := bk.Steps
	p0, ok := findDockerPlugin(steps[0].(map[string]any)["plugins"].([]any))
	if !ok {
		t.Errorf("docker plugin not found in step 0")
	}
	if v, ok := stringInMap(p0, "image"); v != premergeImage || !ok {
		t.Errorf("step 0: got job-env %q, %v, want %q", v, ok, premergeImage)
	}

	p2, ok := findDockerPlugin(steps[2].(map[string]any)["plugins"].([]any))
	if !ok {
		t.Errorf("docker plugin not found in step 2")
	}
	if v, ok := stringInMap(p2, "image"); v != postmergeImage || !ok {
		t.Errorf("step 2: got job-env %q, %v, want %q", v, ok, postmergeImage)
	}
}

func TestConvertPipelineGroup_dockerPlugin(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		buildID:        buildID,
		launcherBranch: "beta",
		gitCommit:      "abcdefg1234567890",
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		CIWorkRepo:      "fakeecr",

		RunnerQueues: map[string]string{"default": "fakerunner"},

		Env: map[string]string{
			"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
		},

		DockerPlugin: &dockerPluginConfig{
			AllowMountBuildkiteAgent: true,
		},
	}, info)

	g := &pipelineGroup{
		Group: "fancy",
		Steps: []map[string]any{{
			"commands":              []string{"has agent"},
			"mount_buildkite_agent": true,
		}, {
			"commands":              []string{"has no agent"},
			"mount_buildkite_agent": false,
		}},
	}
	filter := &stepFilter{runAll: true}
	bk, err := convertSingleGroup(c, g, filter)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if len(bk.Steps) != 2 {
		t.Fatalf("convertPipelineGroup: got %d steps, want 3", len(bk.Steps))
	}

	steps := bk.Steps

	p0, ok := findDockerPlugin(steps[0].(map[string]any)["plugins"].([]any))
	if !ok {
		t.Errorf("docker plugin not found in step 0")
		return
	}
	v, ok := boolInMap(p0, "mount-buildkite-agent")
	if v != true || ok != true {
		t.Errorf("step 0: got docker mount bk agent %v, %v, want true", v, ok)
	}

	p1, ok := findDockerPlugin(steps[1].(map[string]any)["plugins"].([]any))
	if !ok {
		t.Errorf("docker plugin not found in step 0")
		return
	}
	v, ok = boolInMap(p1, "mount-buildkite-agnet")
	if v != false || ok != false {
		t.Errorf("step 1: got docker mount bk agent %v, %v, want false", v, ok)
	}
}

func TestConvertPipelineGroup_awsAssumeRole(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		buildID: buildID,
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		CIWorkRepo:      "fakeecr",

		RunnerQueues: map[string]string{"default": "fakerunner"},
	}, info)

	const role = "arn:aws:iam::123456789012:role/test-role"

	g := &pipelineGroup{
		Group: "fancy",
		Steps: []map[string]any{{
			"commands": []string{"echo 1"},

			"aws_assume_role":                  role,
			"aws_assume_role_duration_seconds": 3600,
		}},
	}

	filter := &stepFilter{runAll: true}
	bk, err := convertSingleGroup(c, g, filter)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	plugins := bk.Steps[0].(map[string]any)["plugins"].([]any)
	assumeRole, ok := findAWSAssumeRolePlugin(plugins)
	if !ok {
		t.Errorf("aws assume role plugin not found in step 0")
	} else {
		if v, _ := stringInMap(assumeRole, "role"); v != role {
			t.Errorf("step 0: got aws assume role %q, want %q", v, role)
		}
		if v, _ := intInMap(assumeRole, "duration"); v != 3600 {
			t.Errorf("step 0: got aws assume role duration %q, want 3600", v)
		}
	}

	docker, ok := findDockerPlugin(plugins)
	if !ok {
		t.Errorf("docker plugin not found in step 0")
	} else if v, _ := boolInMap(docker, "propagate-aws-auth-tokens"); !v {
		t.Errorf("step 0: docker plugin does not have propagate-aws-auth-tokens set")
	}
}

func TestConvertPipelineGroup_awsAssumeRoleDuration(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		buildID: buildID,
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		CIWorkRepo:      "fakeecr",
		RunnerQueues:    map[string]string{"default": "fakerunner"},
	}, info)

	const role = "arn:aws:iam::123456789012:role/test-role"

	for _, test := range []struct {
		name         string
		step         map[string]any
		wantDuration int
	}{
		{
			name: "default duration when not set",
			step: map[string]any{
				"commands":        []string{"echo 1"},
				"aws_assume_role": role,
			},
			wantDuration: 900,
		},
		{
			name: "explicit zero duration preserved",
			step: map[string]any{
				"commands":                         []string{"echo 1"},
				"aws_assume_role":                  role,
				"aws_assume_role_duration_seconds": 0,
			},
			wantDuration: 0,
		},
		{
			name: "explicit duration preserved",
			step: map[string]any{
				"commands":                         []string{"echo 1"},
				"aws_assume_role":                  role,
				"aws_assume_role_duration_seconds": 7200,
			},
			wantDuration: 7200,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := &pipelineGroup{
				Group: "test",
				Steps: []map[string]any{test.step},
			}

			filter := &stepFilter{runAll: true}
			bk, err := convertSingleGroup(c, g, filter)
			if err != nil {
				t.Fatalf("convert: %v", err)
			}

			plugins := bk.Steps[0].(map[string]any)["plugins"].([]any)
			assumeRole, ok := findAWSAssumeRolePlugin(plugins)
			if !ok {
				t.Fatalf("aws assume role plugin not found")
			}

			if got, _ := intInMap(assumeRole, "duration"); got != test.wantDuration {
				t.Errorf("duration = %d, want %d", got, test.wantDuration)
			}
		})
	}
}

func TestConvertPipelineGroup(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		buildID: buildID,
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",

		RunnerQueues: map[string]string{"default": "runner"},
	}, info)

	g := &pipelineGroup{
		Group:     "fancy",
		DependsOn: []string{"forge"},
		Steps: []map[string]any{
			{"commands": []string{"echo 1"}},
			{"wait": nil},
			{"commands": []string{"echo 1"}, "tags": []interface{}{"foo"}},
			{
				"name":  "panda",
				"wanda": "panda.yaml",
				"tags":  []interface{}{"bar"},
			},
			{"commands": []string{"echo 2"}, "tags": []interface{}{"bar"}},
			{"commands": []string{"exit 1"}, "tags": "disabled"},
		},
	}

	filter := &stepFilter{
		skipTags: stringSet("disabled"),
		tags:     stringSet("foo"),
	}
	bk, err := convertSingleGroup(c, g, filter)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if bk.Group != "fancy" {
		t.Errorf("convertPipelineGroup: got group %s, want fancy", bk.Group)
	}
	if want := []string{"forge"}; !reflect.DeepEqual(bk.DependsOn, want) {
		t.Errorf(
			"convertPipelineGroup: got depends_on %+v, want %+v",
			bk.DependsOn, want,
		)
	}
	if len(bk.Steps) != 2 {
		t.Errorf("convertPipelineGroup: got %d steps, want 2", len(bk.Steps))
	}
}

func TestConvertPipelineGroups(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		buildID: buildID,
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",

		RunnerQueues: map[string]string{"default": "runner"},
	}, info)

	groups := []*pipelineGroup{{
		Group:     "fancy",
		DependsOn: []string{"forge"},
		Steps: []map[string]any{
			{"commands": []string{"echo 1"}, "key": "fancy-init"},
			{"wait": nil, "depends_on": "fancy-init"},
			{"commands": []string{"echo 1"}, "tags": []interface{}{"foo"}},
			{
				"name":  "panda",
				"wanda": "panda.yaml",
				"tags":  []interface{}{"bar"},
			},
			{"commands": []string{"echo 2"}, "tags": []interface{}{"bar"}},
			{"commands": []string{"unreachable"}, "depends_on": "no"},
			{"commands": []string{"exit 1"}, "tags": "disabled", "key": "no"},
		},
	}, {
		Group: "failing",
		Tags:  []string{"disabled"},
		Steps: []map[string]any{
			{"commands": []string{"echo bad"}, "key": "bad"},
			{"commands": []string{"echo innocent"}},
		},
	}, {
		Group: "deps",
		Steps: []map[string]any{{
			"commands":   []string{"echo deps bad"},
			"depends_on": []string{"bad"},
			"tags":       []interface{}{"foo"},
		}},
	}, {
		Group: "selectall",
		Tags:  []string{"always"},
		Steps: []map[string]any{{
			"commands": []string{"step 1"},
		}, {
			"commands": []string{"step 2"},
		}},
	}}

	filter := &stepFilter{
		skipTags: stringSet("disabled"),
		tags:     stringSet("foo", "always"),
	}
	bk, err := c.convertGroups(groups, filter)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if len(bk) != 2 {
		t.Fatalf("convertPipelineGroups: got %d groups, want 2", len(bk))
	}

	bk0 := bk[0]

	if bk0.Group != "fancy" {
		t.Errorf("convertPipelineGroup: got group %s, want fancy", bk0.Group)
	}
	if want := []string{"forge"}; !reflect.DeepEqual(bk0.DependsOn, want) {
		t.Errorf(
			"convertPipelineGroup: got depends_on %+v, want %+v",
			bk0.DependsOn, want,
		)
	}
	if len(bk0.Steps) != 3 {
		t.Errorf("convertPipelineGroup: got %d steps, want 3", len(bk0.Steps))
	}

	bk1 := bk[1]
	if bk1.Group != "selectall" {
		t.Errorf("convertPipelineGroup: got group %s, want selectall", bk1.Group)
	}
	if len(bk1.Steps) != 2 {
		t.Errorf("convertPipelineGroup: got %d steps, want 2", len(bk1.Steps))
	}
}
