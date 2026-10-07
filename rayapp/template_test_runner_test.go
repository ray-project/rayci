package rayapp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
)

func newDefaultFake() *fakeAnyscale {
	return &fakeAnyscale{
		defaultCloud:   &fakeCloud{Name: "test-cloud", ID: "cld_test"},
		defaultProject: &fakeProject{Name: "test-project", ID: "prj_test"},
	}
}

func newTestCLI(fake *fakeAnyscale) *AnyscaleCLI {
	cli := NewAnyscaleCLI()
	cli.setRunFunc(fake.run)
	return cli
}

func newFakeAnyscaleAPI(t *testing.T) *anyscaleAPI {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
		},
	))
	t.Cleanup(server.Close)

	api, err := newAnyscaleAPI(server.URL, "test-token")
	if err != nil {
		t.Fatalf("newFakeAnyscaleAPI: %v", err)
	}
	return api
}

func Test_newWorkspaceTestConfig(t *testing.T) {
	tests := []struct {
		name     string
		tmplName string
		buildDir string
	}{
		{
			name:     "basic config",
			tmplName: "my-template",
			buildDir: "path/to",
		},
		{
			name:     "empty values",
			tmplName: "",
			buildDir: "",
		},
		{
			name:     "special characters",
			tmplName: "template-with-dashes_and_underscores",
			buildDir: "/path/with spaces",
		},
	}

	cli := NewAnyscaleCLI()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &Template{Name: tt.tmplName}
			config := newWorkspaceTestConfig(tmpl, cli, nil, tt.buildDir, TestTarget{}, false)

			if config == nil {
				t.Fatal("expected non-nil WorkspaceTestConfig")
			}
			if config.tmplName != tt.tmplName {
				t.Errorf("tmplName = %q, want %q", config.tmplName, tt.tmplName)
			}
			if config.template.Name != tt.tmplName {
				t.Errorf("template.Name = %q, want %q", config.template.Name, tt.tmplName)
			}
			if tmpl.Dir != "" {
				t.Errorf("original template Dir should be unchanged, got %q", tmpl.Dir)
			}
			if config.buildDir != tt.buildDir {
				t.Errorf("buildDir = %q, want %q", config.buildDir, tt.buildDir)
			}
			if !strings.HasPrefix(config.workspaceName, tt.tmplName) {
				t.Errorf("workspaceName %q should start with %q", config.workspaceName, tt.tmplName)
			}
		})
	}
}

func TestWorkspaceTestConfigRun(t *testing.T) {
	tests := []struct {
		name          string
		commandErrors map[string]error
		wantErr       string
	}{
		{
			name: "create workspace fails",
			commandErrors: map[string]error{
				"workspace_v2 create": fmt.Errorf("create failed"),
			},
			wantErr: "create empty workspace failed",
		},
		{
			name: "get workspace ID fails",
			commandErrors: map[string]error{
				"workspace_v2 get": fmt.Errorf("get failed"),
			},
			wantErr: "get workspace ID failed",
		},
		{
			name: "start workspace fails",
			commandErrors: map[string]error{
				"workspace_v2 start": fmt.Errorf("start failed"),
			},
			wantErr: "start workspace failed",
		},
		{
			name: "wait for state fails",
			commandErrors: map[string]error{
				"workspace_v2 wait": fmt.Errorf("wait failed"),
			},
			wantErr: "wait for workspace running state failed",
		},
		{
			name: "push template fails",
			commandErrors: map[string]error{
				"workspace_v2 push": fmt.Errorf("push failed"),
			},
			wantErr: "push template to workspace failed",
		},
		{
			name: "unzip fails",
			commandErrors: map[string]error{
				"workspace_v2 run_command": fmt.Errorf("run failed"),
			},
			wantErr: "unzip template failed",
		},
		{
			name: "terminate fails",
			commandErrors: map[string]error{
				"workspace_v2 terminate": fmt.Errorf("terminate failed"),
			},
			wantErr: "terminate workspace failed",
		},
		{
			name:    "success",
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newDefaultFake()
			fake.commandErrors = tt.commandErrors
			cli := newTestCLI(fake)
			api := newFakeAnyscaleAPI(t)

			err := runTemplateTestsWithFilter(
				"testdata/BUILD.yaml",
				func(tmpl *Template) bool { return tmpl.Name == "fishy-ray" },
				"", false, TestTarget{}, cli, api,
			)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestWorkspaceTestConfigRun_EscapesSingleQuotes(t *testing.T) {
	fake := newDefaultFake()
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	var capturedCmd string
	cli.setRunFunc(func(args []string) (string, error) {
		cmd := fmt.Sprintf("%s %s", args[0], args[1])
		if cmd == "workspace_v2 run_command" && strings.HasPrefix(args[4], "timeout") {
			capturedCmd = args[4]
		}
		return fake.run(args)
	})

	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "fishy-ray" },
		"", false, TestTarget{}, cli, api,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedCmd, "bash -c '") {
		t.Errorf("command %q should contain bash -c invocation", capturedCmd)
	}
}

func TestWorkspaceTestConfigRun_TestCommandFails(t *testing.T) {
	fake := newDefaultFake()
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	cli.setRunFunc(func(args []string) (string, error) {
		cmd := fmt.Sprintf("%s %s", args[0], args[1])
		if cmd == "workspace_v2 run_command" && strings.HasPrefix(args[4], "timeout") {
			return "", fmt.Errorf("test execution failed")
		}
		return fake.run(args)
	})

	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "fishy-ray" },
		"", false, TestTarget{}, cli, api,
	)
	if err == nil {
		t.Fatal("expected error when test command fails")
	}
	if !strings.Contains(err.Error(), "run test command failed") {
		t.Errorf("error %q should contain 'run test command failed'", err.Error())
	}
}

func TestRunTemplateTest_Failure(t *testing.T) {
	fake := newDefaultFake()
	fake.commandErrors = map[string]error{
		"compute-config list": fmt.Errorf("forced failure"),
	}
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "fishy-ray" },
		"", false, TestTarget{}, cli, api,
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "test failed") {
		t.Errorf("error %q should contain 'test failed'", err.Error())
	}
	if !strings.Contains(err.Error(), "fishy-ray") {
		t.Errorf("error %q should contain template name 'fishy-ray'", err.Error())
	}
}

func TestRunTemplateTest_SkipsTemplateWithNoTestConfig(t *testing.T) {
	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "reefy-ray" },
		"", false, TestTarget{}, nil, nil,
	)
	if err != nil {
		t.Fatalf("expected no error when matched template has no test config, got: %v", err)
	}
}

func TestRunTemplateTest_NoTemplatesToTest(t *testing.T) {
	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "nonexistent-template" },
		"", false, TestTarget{}, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error when no templates match filter")
	}
	if !strings.Contains(err.Error(), "no templates to test") {
		t.Errorf("error %q should contain 'no templates to test'", err.Error())
	}
}

func TestRunTemplateTest_ReadTemplatesFailed(t *testing.T) {
	err := runTemplateTestsWithFilter(
		"nonexistent/BUILD.yaml",
		nil,
		"",
		false,
		TestTarget{},
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected error for invalid build file")
	}
	if !strings.Contains(err.Error(), "read templates failed") {
		t.Errorf("error %q should contain 'read templates failed'", err.Error())
	}
}

func TestRunTemplateTest_FilterSelectsSingleTemplate(t *testing.T) {
	fake := newDefaultFake()
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "fishy-ray" },
		"", false, TestTarget{}, cli, api,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAllTemplateTests_Success(t *testing.T) {
	fake := newDefaultFake()
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	err := runTemplateTestsWithFilter("testdata/BUILD.yaml", nil, "", false, TestTarget{}, cli, api)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAllTemplateTests_NoTemplatesToTest(t *testing.T) {
	f := createEmptyBuildFile(t)

	err := runTemplateTestsWithFilter(f, nil, "", false, TestTarget{}, nil, nil)
	if err == nil {
		t.Fatal("expected error when build file has no templates")
	}
	if !strings.Contains(err.Error(), "no templates to test") {
		t.Errorf("error %q should contain 'no templates to test'", err.Error())
	}
}

func TestRunAllTemplateTests_PartialFailure(t *testing.T) {
	fake := newDefaultFake()
	fake.commandErrors = map[string]error{
		"compute-config list": fmt.Errorf("forced failure"),
	}
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	err := runTemplateTestsWithFilter("testdata/BUILD.yaml", nil, "", false, TestTarget{}, cli, api)
	if err == nil {
		t.Fatal("expected error when some templates fail")
	}
	if !strings.Contains(err.Error(), "test failed for templates") {
		t.Errorf("error %q should contain 'test failed for templates'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_WithTestsPath(t *testing.T) {
	fake := newDefaultFake()
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "testy-ray" },
		"", false, TestTarget{}, cli, api,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func createEmptyBuildFile(t *testing.T) string {
	t.Helper()
	f := fmt.Sprintf("%s/BUILD.yaml", t.TempDir())
	if err := os.WriteFile(f, []byte("[]"), 0644); err != nil {
		t.Fatalf("create empty build file: %v", err)
	}
	return f
}

// newProbeTestAPI creates a fake Anyscale API server that handles both
// POST /from_template and DELETE requests.
func newProbeTestAPI(t *testing.T, launchResult map[string]any, launchStatus int) *anyscaleAPI {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/from_template"):
				if launchStatus != 0 {
					w.WriteHeader(launchStatus)
					w.Write([]byte(`{"error":"launch failed"}`))
					return
				}
				resp := map[string]any{"result": launchResult}
				bs, _ := json.Marshal(resp)
				w.WriteHeader(http.StatusOK)
				w.Write(bs)
			case r.Method == http.MethodDelete:
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("{}"))
			default:
				http.Error(w, "not found", http.StatusNotFound)
			}
		},
	))
	t.Cleanup(server.Close)

	api, err := newAnyscaleAPI(server.URL, "test-token")
	if err != nil {
		t.Fatalf("newProbeTestAPI: %v", err)
	}
	return api
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name          string
		commandErrors map[string]error
		launchResult  map[string]any
		launchStatus  int
		wantErr       string
	}{
		{
			name: "get default cloud fails",
			commandErrors: map[string]error{
				"cloud get-default": fmt.Errorf("cloud error"),
			},
			wantErr: "get default cloud failed",
		},
		{
			name: "get default project fails",
			commandErrors: map[string]error{
				"project get-default": fmt.Errorf("project error"),
			},
			wantErr: "get default project failed",
		},
		{
			name:         "launch template API fails",
			launchStatus: http.StatusInternalServerError,
			wantErr:      "launch template in workspace failed",
		},
		{
			name: "unexpected response format",
			launchResult: map[string]any{
				"other": "data",
			},
			wantErr: "unexpected response format: missing name or id",
		},
		{
			name: "wait for running state fails",
			launchResult: map[string]any{
				"name": "ws-test",
				"id":   "expwrk_test",
			},
			commandErrors: map[string]error{
				"workspace_v2 wait": fmt.Errorf("wait failed"),
			},
			wantErr: "wait for workspace running state failed",
		},
		{
			name: "success",
			launchResult: map[string]any{
				"name": "ws-test",
				"id":   "expwrk_test",
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newDefaultFake()
			fake.commandErrors = tt.commandErrors
			cli := newTestCLI(fake)
			api := newProbeTestAPI(t, tt.launchResult, tt.launchStatus)

			err := probe("fishy-ray", "testdata/BUILD.yaml", TestTarget{}, cli, api)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestProbe_CleanupFails(t *testing.T) {
	fake := newDefaultFake()
	fake.commandErrors = map[string]error{
		"workspace_v2 terminate": fmt.Errorf("terminate failed"),
	}
	cli := newTestCLI(fake)
	api := newProbeTestAPI(t, map[string]any{
		"name": "ws-test",
		"id":   "expwrk_test",
	}, 0)

	err := probe("fishy-ray", "testdata/BUILD.yaml", TestTarget{}, cli, api)
	if err == nil {
		t.Fatal("expected error when cleanup fails")
	}
	if !strings.Contains(err.Error(), "terminate workspace failed") {
		t.Errorf("error %q should contain 'terminate workspace failed'", err.Error())
	}
}

func TestProbe_RunsTestCommand(t *testing.T) {
	fake := newDefaultFake()
	cli := newTestCLI(fake)
	api := newProbeTestAPI(t, map[string]any{
		"name": "ws-test",
		"id":   "expwrk_test",
	}, 0)

	var capturedCmd string
	cli.setRunFunc(func(args []string) (string, error) {
		cmd := fmt.Sprintf("%s %s", args[0], args[1])
		if cmd == "workspace_v2 run_command" && strings.HasPrefix(args[4], "timeout") {
			capturedCmd = args[4]
		}
		return fake.run(args)
	})

	err := probe("fishy-ray", "testdata/BUILD.yaml", TestTarget{}, cli, api)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedCmd == "" {
		t.Fatal("expected test command to be executed")
	}
	if !strings.Contains(capturedCmd, "bash -c '") {
		t.Errorf("command %q should contain bash -c invocation", capturedCmd)
	}
}

func TestProbe_TestCommandFails(t *testing.T) {
	fake := newDefaultFake()
	cli := newTestCLI(fake)
	api := newProbeTestAPI(t, map[string]any{
		"name": "ws-test",
		"id":   "expwrk_test",
	}, 0)

	cli.setRunFunc(func(args []string) (string, error) {
		cmd := fmt.Sprintf("%s %s", args[0], args[1])
		if cmd == "workspace_v2 run_command" && strings.HasPrefix(args[4], "timeout") {
			return "", fmt.Errorf("test execution failed")
		}
		return fake.run(args)
	})

	err := probe("fishy-ray", "testdata/BUILD.yaml", TestTarget{}, cli, api)
	if err == nil {
		t.Fatal("expected error when test command fails")
	}
	if !strings.Contains(err.Error(), "run test command failed") {
		t.Errorf("error %q should contain 'run test command failed'", err.Error())
	}
}

func TestProbe_WithTestsPath(t *testing.T) {
	fake := newDefaultFake()
	cli := newTestCLI(fake)
	api := newProbeTestAPI(t, map[string]any{
		"name": "ws-test",
		"id":   "expwrk_test",
	}, 0)

	err := probe("testy-ray", "testdata/BUILD.yaml", TestTarget{}, cli, api)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTemplateTestsWithRayVersionOverride(t *testing.T) {
	tests := []struct {
		name         string
		tmplName     string
		rayVersion   string
		wantImageURI string
	}{
		{
			name:         "override build_id template",
			tmplName:     "fishy-ray",
			rayVersion:   "2.44.0",
			wantImageURI: "anyscale/ray:2.44.0-py311",
		},
		{
			name:         "override image_uri template",
			tmplName:     "image-uri-ray",
			rayVersion:   "2.44.0",
			wantImageURI: "anyscale/ray:2.44.0-py311",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newDefaultFake()
			cli := newTestCLI(fake)
			api := newFakeAnyscaleAPI(t)

			var capturedImageURI string
			cli.setRunFunc(func(args []string) (string, error) {
				cmd := fmt.Sprintf("%s %s", args[0], args[1])
				if cmd == "workspace_v2 create" {
					for i, arg := range args {
						if arg == "--image-uri" && i+1 < len(args) {
							capturedImageURI = args[i+1]
						}
					}
				}
				return fake.run(args)
			})

			err := runTemplateTestsWithFilter(
				"testdata/BUILD.yaml",
				func(tmpl *Template) bool { return tmpl.Name == tt.tmplName },
				tt.rayVersion, false, TestTarget{}, cli, api,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedImageURI != tt.wantImageURI {
				t.Errorf("image URI = %q, want %q", capturedImageURI, tt.wantImageURI)
			}
		})
	}
}

func TestRunTemplateTestsWithFilter_InvalidRayVersion(t *testing.T) {
	invalidVersions := []struct {
		name       string
		rayVersion string
	}{
		{"missing minor digit", "2.4.0"},
		{"no dots", "2440"},
		{"extra prefix", "v2.44.0"},
		{"trailing text", "2.44.0-rc1"},
		{"two-digit major", "22.44.0"},
	}

	for _, tt := range invalidVersions {
		t.Run(tt.name, func(t *testing.T) {
			err := runTemplateTestsWithFilter(
				"testdata/BUILD.yaml", nil, tt.rayVersion, false, TestTarget{}, nil, nil,
			)
			if err == nil {
				t.Fatal("expected error for invalid ray version")
			}
			if !strings.Contains(err.Error(), "invalid ray version") {
				t.Errorf("error %q should contain 'invalid ray version'", err.Error())
			}
		})
	}
}

func TestRunTemplateTestsWithRayVersionOverride_SkipsBYODImage(t *testing.T) {
	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "byod-ray" },
		"2.44.0", false, TestTarget{}, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error when BYOD image is skipped")
	}
	if !strings.Contains(err.Error(), "no templates to test") {
		t.Errorf("error %q should contain 'no templates to test'", err.Error())
	}
}

func TestRunTemplateTestsWithRayVersionOverride_SkipsNonRayImage(t *testing.T) {
	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "custom-image" },
		"2.44.0", false, TestTarget{}, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error when non-ray image_uri is skipped")
	}
	if !strings.Contains(err.Error(), "no templates to test") {
		t.Errorf("error %q should contain 'no templates to test'", err.Error())
	}
}

func TestRunTemplateTestsWithNightlyOverride(t *testing.T) {
	tests := []struct {
		name         string
		tmplName     string
		wantImageURI string
	}{
		{
			name:         "nightly override build_id template",
			tmplName:     "fishy-ray",
			wantImageURI: "anyscale/ray:nightly-py311",
		},
		{
			name:         "nightly override image_uri template",
			tmplName:     "image-uri-ray",
			wantImageURI: "anyscale/ray:nightly-py311",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newDefaultFake()
			cli := newTestCLI(fake)
			api := newFakeAnyscaleAPI(t)

			var capturedImageURI string
			cli.setRunFunc(func(args []string) (string, error) {
				cmd := fmt.Sprintf("%s %s", args[0], args[1])
				if cmd == "workspace_v2 create" {
					for i, arg := range args {
						if arg == "--image-uri" && i+1 < len(args) {
							capturedImageURI = args[i+1]
						}
					}
				}
				return fake.run(args)
			})

			err := runTemplateTestsWithFilter(
				"testdata/BUILD.yaml",
				func(tmpl *Template) bool { return tmpl.Name == tt.tmplName },
				"", true, TestTarget{}, cli, api,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedImageURI != tt.wantImageURI {
				t.Errorf("image URI = %q, want %q", capturedImageURI, tt.wantImageURI)
			}
		})
	}
}

func TestRunTemplateTestsWithNightlyOverride_SkipsBYODImage(t *testing.T) {
	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "byod-ray" },
		"", true, TestTarget{}, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error when BYOD image is skipped")
	}
	if !strings.Contains(err.Error(), "no templates to test") {
		t.Errorf("error %q should contain 'no templates to test'", err.Error())
	}
}

func TestRunTemplateTestsWithNightlyOverride_SkipsNonRayImage(t *testing.T) {
	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "custom-image" },
		"", true, TestTarget{}, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error when non-ray image_uri is skipped")
	}
	if !strings.Contains(err.Error(), "no templates to test") {
		t.Errorf("error %q should contain 'no templates to test'", err.Error())
	}
}

func TestRunTemplateTestsWithFilter_NightlyAndRayVersionMutuallyExclusive(t *testing.T) {
	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml", nil, "2.44.0", true, TestTarget{}, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error when both nightly and ray-version are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error %q should contain 'mutually exclusive'", err.Error())
	}
}

func TestProbe_TemplateNotFound(t *testing.T) {
	err := probe("nonexistent", "testdata/BUILD.yaml", TestTarget{}, nil, nil)
	if err == nil {
		t.Fatal("expected error when template not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should contain 'not found'", err.Error())
	}
}

func TestProbe_ReadTemplatesFails(t *testing.T) {
	err := probe("any", "nonexistent/BUILD.yaml", TestTarget{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid build file")
	}
	if !strings.Contains(err.Error(), "read templates failed") {
		t.Errorf("error %q should contain 'read templates failed'", err.Error())
	}
}

func TestRunTemplateTestsWithTarget(t *testing.T) {
	fake := newDefaultFake()
	var configName, configPath, configCloud string
	fake.onCreateComputeConfig = func(args []string) (string, error) {
		configName = parseFlag(args, "-n")
		configPath = args[len(args)-1]
		data, err := os.ReadFile(configPath)
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if v, ok := strings.CutPrefix(line, "cloud: "); ok {
				configCloud = v
			}
		}
		return "created compute config", nil
	}
	var wsArgs []string
	fake.onCreateWorkspace = func(args []string) (string, error) {
		wsArgs = args
		return fake.workspaceCreate(args[2:])
	}
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "fishy-ray" },
		"", false,
		TestTarget{Cloud: "k8s cloud", ComputeConfig: "GCP"},
		cli, api,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The cloud is slugified into the name so configs created from the same
	// file in different clouds do not collide.
	if want := "configs-gcp-k8s-cloud"; configName != want {
		t.Errorf("compute config name = %q, want %q", configName, want)
	}
	if configCloud != "k8s cloud" {
		t.Errorf("compute config cloud = %q, want %q", configCloud, "k8s cloud")
	}
	checkArgs(t, wsArgs,
		[]string{"workspace_v2", "create"}, nil,
		[][2]string{
			{"--cloud", "k8s cloud"},
			{"--compute-config", "configs-gcp-k8s-cloud"},
		},
	)
}

func TestRunTemplateTestsWithTarget_DefaultsToAWS(t *testing.T) {
	fake := newDefaultFake()
	var configName string
	fake.onCreateComputeConfig = func(args []string) (string, error) {
		configName = parseFlag(args, "-n")
		return "created compute config", nil
	}
	var wsArgs []string
	fake.onCreateWorkspace = func(args []string) (string, error) {
		wsArgs = args
		return fake.workspaceCreate(args[2:])
	}
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "fishy-ray" },
		"", false, TestTarget{}, cli, api,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "configs-aws"; configName != want {
		t.Errorf("compute config name = %q, want %q", configName, want)
	}
	if slices.Contains(wsArgs, "--cloud") {
		t.Errorf("workspace create args %v should not set --cloud", wsArgs)
	}
}

func TestRunTemplateTestsWithTarget_MissingComputeConfig(t *testing.T) {
	fake := newDefaultFake()
	fake.onCreateWorkspace = func(args []string) (string, error) {
		t.Errorf("workspace should not be created, got %v", args)
		return "", nil
	}
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "fishy-ray" },
		"", false, TestTarget{ComputeConfig: "K8S"}, cli, api,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `template "fishy-ray" has no "K8S" compute config`) {
		t.Errorf("error %q should name the missing compute config", err.Error())
	}
}

func TestProbe_WithCloud(t *testing.T) {
	tests := []struct {
		name        string
		cloud       string
		wantCloudID string
		wantErr     string
	}{
		{name: "default cloud", wantCloudID: "cld_test"},
		{name: "named cloud", cloud: "k8s-cloud", wantCloudID: "cld_k8s"},
		{name: "unknown cloud", cloud: "missing", wantErr: `cloud "missing" not found`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newDefaultFake()
			fake.clouds = []*fakeCloud{{Name: "k8s-cloud", ID: "cld_k8s"}}
			cli := newTestCLI(fake)

			var gotCloudID string
			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPost {
						var payload map[string]any
						json.NewDecoder(r.Body).Decode(&payload)
						gotCloudID, _ = payload["cloud_id"].(string)
						w.Write([]byte(`{"result":{"name":"ws-test","id":"expwrk_test"}}`))
						return
					}
					w.Write([]byte("{}"))
				},
			))
			t.Cleanup(server.Close)
			api, err := newAnyscaleAPI(server.URL, "test-token")
			if err != nil {
				t.Fatal(err)
			}

			err = probe(
				"fishy-ray", "testdata/BUILD.yaml",
				TestTarget{Cloud: tt.cloud}, cli, api,
			)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotCloudID != tt.wantCloudID {
				t.Errorf("launched in cloud %q, want %q", gotCloudID, tt.wantCloudID)
			}
		})
	}
}
