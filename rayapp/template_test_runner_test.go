package rayapp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newDefaultFake() *fakeAnyscale {
	return &fakeAnyscale{
		defaultCloud: &fakeCloud{Name: "test-cloud", ID: "cld_test"},
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

func TestNewWorkspaceTestConfig(t *testing.T) {
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
			config := NewWorkspaceTestConfig(tmpl, cli, nil, tt.buildDir)

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
				t.Errorf(
					"workspaceName %q should start with %q",
					config.workspaceName, tt.tmplName,
				)
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
			name: "push zip fails",
			commandErrors: map[string]error{
				"workspace_v2 push": fmt.Errorf("push failed"),
			},
			wantErr: "push zip to workspace failed",
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
				func(tmpl *Template) bool { return tmpl.Name == "reefy-ray" },
				cli, api,
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

func TestWorkspaceTestConfigRun_TestCommandFails(t *testing.T) {
	fake := newDefaultFake()
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	var runCmdCount int
	cli.setRunFunc(func(args []string) (string, error) {
		cmd := fmt.Sprintf("%s %s", args[0], args[1])
		if cmd == "workspace_v2 run_command" {
			runCmdCount++
			if runCmdCount > 1 {
				return "", fmt.Errorf("test execution failed")
			}
		}
		return fake.run(args)
	})

	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "reefy-ray" },
		cli, api,
	)
	if err == nil {
		t.Fatal("expected error when test command fails")
	}
	if !strings.Contains(err.Error(), "test command failed") {
		t.Errorf("error %q should contain 'test command failed'", err.Error())
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
		func(tmpl *Template) bool { return tmpl.Name == "reefy-ray" },
		cli, api,
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "test failed") {
		t.Errorf("error %q should contain 'test failed'", err.Error())
	}
	if !strings.Contains(err.Error(), "reefy-ray") {
		t.Errorf("error %q should contain template name 'reefy-ray'", err.Error())
	}
}

func TestRunTemplateTest_NoTemplatesToTest(t *testing.T) {
	err := runTemplateTestsWithFilter(
		"testdata/BUILD.yaml",
		func(tmpl *Template) bool { return tmpl.Name == "nonexistent-template" },
		nil, nil,
	)
	if err == nil {
		t.Fatal("expected error when no templates match filter")
	}
	if !strings.Contains(err.Error(), "no templates to test") {
		t.Errorf("error %q should contain 'no templates to test'", err.Error())
	}
}

func TestRunTemplateTest_ReadTemplatesFailed(t *testing.T) {
	err := runTemplateTestsWithFilter("nonexistent/BUILD.yaml", nil, nil, nil)
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
		cli, api,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAllTemplateTests_Success(t *testing.T) {
	fake := newDefaultFake()
	cli := newTestCLI(fake)
	api := newFakeAnyscaleAPI(t)

	err := runTemplateTestsWithFilter("testdata/BUILD.yaml", nil, cli, api)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAllTemplateTests_NoTemplatesToTest(t *testing.T) {
	f := createEmptyBuildFile(t)

	err := runTemplateTestsWithFilter(f, nil, nil, nil)
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

	err := runTemplateTestsWithFilter("testdata/BUILD.yaml", nil, cli, api)
	if err == nil {
		t.Fatal("expected error when some templates fail")
	}
	if !strings.Contains(err.Error(), "test failed for templates") {
		t.Errorf("error %q should contain 'test failed for templates'", err.Error())
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

func TestTestCmd_Constant(t *testing.T) {
	if testCmd == "" {
		t.Error("testCmd should not be empty")
	}
	if !strings.Contains(testCmd, "pytest") {
		t.Errorf("testCmd %q should contain 'pytest'", testCmd)
	}
	if !strings.Contains(testCmd, "nbmake") {
		t.Errorf("testCmd %q should contain 'nbmake'", testCmd)
	}
}
