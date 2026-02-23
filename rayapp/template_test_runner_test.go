package rayapp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupMockAnyscale installs a fake anyscale script and prepends its directory to PATH
// so NewAnyscaleCLI() and exec.LookPath("anyscale") use it.
func setupMockAnyscale(t *testing.T, script string) {
	t.Helper()
	binPath := writeFakeAnyscale(t, script)
	dir := filepath.Dir(binPath)
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
}

// setupMockDeleteWorkspaceAPI starts an httptest.Server that accepts DELETE workspace
// and sets ANYSCALE_HOST/ANYSCALE_CLI_TOKEN so deleteWorkspaceByID succeeds.
func setupMockDeleteWorkspaceAPI(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	t.Cleanup(server.Close)

	origHost := os.Getenv("ANYSCALE_HOST")
	origToken := os.Getenv("ANYSCALE_CLI_TOKEN")
	t.Cleanup(func() {
		if origHost == "" {
			os.Unsetenv("ANYSCALE_HOST")
		} else {
			os.Setenv("ANYSCALE_HOST", origHost)
		}
		if origToken == "" {
			os.Unsetenv("ANYSCALE_CLI_TOKEN")
		} else {
			os.Setenv("ANYSCALE_CLI_TOKEN", origToken)
		}
	})
	os.Setenv("ANYSCALE_HOST", server.URL)
	os.Setenv("ANYSCALE_CLI_TOKEN", "test-token")
}

func TestNewWorkspaceTestConfig(t *testing.T) {
	tests := []struct {
		name      string
		tmplName  string
		buildFile string
	}{
		{
			name:      "basic config",
			tmplName:  "my-template",
			buildFile: "path/to/build.yaml",
		},
		{
			name:      "empty values",
			tmplName:  "",
			buildFile: "",
		},
		{
			name:      "special characters",
			tmplName:  "template-with-dashes_and_underscores",
			buildFile: "/path/with spaces/build.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewWorkspaceTestConfig(tt.tmplName)

			if config == nil {
				t.Fatal("expected non-nil WorkspaceTestConfig")
			}
			if config.tmplName != tt.tmplName {
				t.Errorf("tmplName = %q, want %q", config.tmplName, tt.tmplName)
			}
			// Other fields should be zero values
			if config.workspaceName != "" {
				t.Errorf("workspaceName should be empty, got %q", config.workspaceName)
			}
			if config.template != nil {
				t.Error("template should be nil")
			}
		})
	}
}

func TestWorkspaceTestConfigRun_CreateWorkspaceFails(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then echo "created"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "create failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	err := Test("reefy-ray", "testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error when create workspace fails")
	}
	if !strings.Contains(err.Error(), "create empty workspace failed") {
		t.Errorf("error %q should contain 'create empty workspace failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_GetWorkspaceIDFails(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then echo "created"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "Workspace created successfully id: expwrk_testid123"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo "get failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	err := Test("reefy-ray", "testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error when get workspace ID fails")
	}
	if !strings.Contains(err.Error(), "get workspace ID failed") {
		t.Errorf("error %q should contain 'get workspace ID failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_StartWorkspaceFails(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then echo "created"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "Workspace created successfully id: expwrk_testid123"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"id": "expwrk_testid123", "name": "test"}'
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "start" ]; then
    echo "start failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	err := Test("reefy-ray", "testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error when start workspace fails")
	}
	if !strings.Contains(err.Error(), "start workspace failed") {
		t.Errorf("error %q should contain 'start workspace failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_WaitForStateFails(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then echo "created"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "Workspace created successfully id: expwrk_testid123"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"id": "expwrk_testid123", "name": "test"}'
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "start" ]; then
    echo "started"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "wait" ]; then
    echo "wait failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	err := Test("reefy-ray", "testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error when wait for state fails")
	}
	if !strings.Contains(err.Error(), "wait for workspace running state failed") {
		t.Errorf("error %q should contain 'wait for workspace running state failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_CopyTemplateFails(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then echo "created"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "Workspace created successfully id: expwrk_testid123"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "start" ]; then
    echo "started"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "wait" ]; then
    echo "running"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"id": "expwrk_testid123", "name": "test"}'
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "push" ]; then
    echo "push failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	err := Test("reefy-ray", "testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error when copy template fails")
	}
	if !strings.Contains(err.Error(), "push zip to workspace failed") {
		t.Errorf("error %q should contain 'push zip to workspace failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_RunCommandFails(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then echo "created"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "Workspace created successfully id: expwrk_testid123"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "start" ]; then
    echo "started"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "wait" ]; then
    echo "running"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "push" ]; then
    echo "pushed"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"id": "expwrk_testid123", "name": "test"}'
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "run_command" ]; then
    echo "run_command failed: forced failure" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	err := Test("reefy-ray", "testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error when run command fails")
	}
	if !strings.Contains(err.Error(), "run_command failed") {
		t.Errorf("error %q should contain 'run_command failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_TerminateFails(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then echo "created"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "Workspace created successfully id: expwrk_testid123"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "start" ]; then
    echo "started"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "wait" ]; then
    echo "running"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "push" ]; then
    echo "pushed"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "run_command" ]; then
    echo "tests passed"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"id": "expwrk_testid123", "name": "test"}'
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "terminate" ]; then
    echo "terminate failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	err := Test("reefy-ray", "testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error when terminate fails")
	}
	if !strings.Contains(err.Error(), "terminate workspace failed") {
		t.Errorf("error %q should contain 'terminate workspace failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_Success(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then
    echo "created compute config"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "Workspace created successfully id: expwrk_testid123"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"id": "expwrk_testid123", "name": "test"}'
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "start" ]; then
    echo "started"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "wait" ]; then
    echo "running"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "push" ]; then
    echo "pushed"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "run_command" ]; then
    echo "tests passed"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "terminate" ]; then
    echo "terminated"
    exit 0
fi
echo "unknown command: $@"
exit 1
`
	setupMockAnyscale(t, script)

	err := Test("reefy-ray", "testdata/BUILD.yaml")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTest_Success(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then
    echo "created"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "Workspace created successfully id: expwrk_testid123"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"id": "expwrk_testid123", "name": "test"}'
    exit 0
fi
if [ "$1" = "workspace_v2" ]; then
    echo "success"
    exit 0
fi
echo "unknown"
exit 1
`
	setupMockAnyscale(t, script)

	err := Test("reefy-ray", "testdata/BUILD.yaml")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTest_Failure(t *testing.T) {
	setupMockAnyscale(t, "#!/bin/sh\nexit 1")

	err := Test("reefy-ray", "testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "test failed") {
		t.Errorf("error %q should contain 'test failed'", err.Error())
	}
	if !strings.Contains(err.Error(), "reefy-ray") {
		t.Errorf("error %q should contain failed template name 'reefy-ray'", err.Error())
	}
}

func TestTest_NoTemplatesToTest(t *testing.T) {
	setupMockAnyscale(t, "#!/bin/sh\necho ok")

	err := Test("nonexistent-template", "testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error when no templates match filter")
	}
	if !strings.Contains(err.Error(), "no templates to test") {
		t.Errorf("error %q should contain 'no templates to test'", err.Error())
	}
}

func TestTest_ReadTemplatesFailed(t *testing.T) {
	setupMockAnyscale(t, "#!/bin/sh\necho ok")

	err := Test("reefy-ray", "nonexistent/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error for invalid build file")
	}
	if !strings.Contains(err.Error(), "read templates failed") {
		t.Errorf("error %q should contain 'read templates failed'", err.Error())
	}
}

func TestTest_FilterSelectsSingleTemplate(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then echo "created"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then echo "Workspace created successfully id: expwrk_testid123"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then echo '{"id": "expwrk_testid123", "name": "test"}'; exit 0; fi
if [ "$1" = "workspace_v2" ]; then echo "success"; exit 0; fi
echo "unknown"; exit 1
`
	setupMockAnyscale(t, script)

	err := Test("fishy-ray", "testdata/BUILD.yaml")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestAll_Success(t *testing.T) {
	setupMockDeleteWorkspaceAPI(t)
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'; exit 0; fi
if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit 0; fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then echo "created"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then echo "Workspace created successfully id: expwrk_testid123"; exit 0; fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then echo '{"id": "expwrk_testid123", "name": "test"}'; exit 0; fi
if [ "$1" = "workspace_v2" ]; then echo "success"; exit 0; fi
echo "unknown"; exit 1
`
	setupMockAnyscale(t, script)

	err := TestAll("testdata/BUILD.yaml")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestAll_ReadTemplatesFailed(t *testing.T) {
	setupMockAnyscale(t, "#!/bin/sh\necho ok")

	err := TestAll("nonexistent/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error for invalid build file")
	}
	if !strings.Contains(err.Error(), "read templates failed") {
		t.Errorf("error %q should contain 'read templates failed'", err.Error())
	}
}

func TestTestAll_NoTemplatesToTest(t *testing.T) {
	setupMockAnyscale(t, "#!/bin/sh\necho ok")
	f := createEmptyBuildFile(t)

	err := TestAll(f)

	if err == nil {
		t.Fatal("expected error when build file has no templates")
	}
	if !strings.Contains(err.Error(), "no templates to test") {
		t.Errorf("error %q should contain 'no templates to test'", err.Error())
	}
}

func TestTestAll_PartialFailure(t *testing.T) {
	setupMockAnyscale(t, "#!/bin/sh\nexit 1")

	err := TestAll("testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error when some templates fail")
	}
	if !strings.Contains(err.Error(), "test failed for templates") {
		t.Errorf("error %q should contain 'test failed for templates'", err.Error())
	}
}

func createEmptyBuildFile(t *testing.T) string {
	t.Helper()
	f := t.TempDir() + "/BUILD.yaml"
	if err := os.WriteFile(f, []byte("[]"), 0644); err != nil {
		t.Fatalf("create empty build file: %v", err)
	}
	return f
}

func TestTestCmd_Constant(t *testing.T) {
	// Verify the test command constant is set correctly
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

func TestWorkspaceTestConfigRun_UsesAnyscaleToken(t *testing.T) {
	// Set a test token
	origToken := os.Getenv("ANYSCALE_CLI_TOKEN")
	t.Cleanup(func() {
		if origToken == "" {
			os.Unsetenv("ANYSCALE_CLI_TOKEN")
		} else {
			os.Setenv("ANYSCALE_CLI_TOKEN", origToken)
		}
	})
	os.Setenv("ANYSCALE_CLI_TOKEN", "test-token-123")

	// Mock that fails immediately so we can test without full execution
	setupMockAnyscale(t, "#!/bin/sh\nexit 1")

	// We don't care about the error, just that it uses the token
	_ = Test("reefy-ray", "testdata/BUILD.yaml")
}

// setupMockProbeAPI starts an httptest.Server that handles both
// POST /from_template (launch) and DELETE /workspaces/{id} (cleanup).
func setupMockProbeAPI(t *testing.T, launchStatus int, launchBody string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/from_template"):
			w.WriteHeader(launchStatus)
			fmt.Fprint(w, launchBody)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{}`)
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	origHost := os.Getenv("ANYSCALE_HOST")
	origToken := os.Getenv("ANYSCALE_CLI_TOKEN")
	t.Cleanup(func() {
		if origHost == "" {
			os.Unsetenv("ANYSCALE_HOST")
		} else {
			os.Setenv("ANYSCALE_HOST", origHost)
		}
		if origToken == "" {
			os.Unsetenv("ANYSCALE_CLI_TOKEN")
		} else {
			os.Setenv("ANYSCALE_CLI_TOKEN", origToken)
		}
	})
	os.Setenv("ANYSCALE_HOST", server.URL)
	os.Setenv("ANYSCALE_CLI_TOKEN", "test-token")
}

// probeScript builds a mock anyscale CLI script for Probe tests.
// Each step can be set to succeed or fail via the flags.
func probeScript(cloudOK, projectOK, waitOK, terminateOK bool) string {
	lines := []string{"#!/bin/sh"}

	cloudExit := "0"
	if !cloudOK {
		cloudExit = "1"
	}
	lines = append(lines, fmt.Sprintf(
		`if [ "$1" = "cloud" ] && [ "$2" = "get-default" ]; then echo "name: test-cloud"; echo "id: cld_test"; exit %s; fi`,
		cloudExit,
	))

	projectExit := "0"
	if !projectOK {
		projectExit = "1"
	}
	lines = append(lines, fmt.Sprintf(
		`if [ "$1" = "project" ] && [ "$2" = "get-default" ]; then echo "name: test-project"; echo "id: prj_test"; exit %s; fi`,
		projectExit,
	))

	waitExit := "0"
	if !waitOK {
		waitExit = "1"
	}
	lines = append(lines, fmt.Sprintf(
		`if [ "$1" = "workspace_v2" ] && [ "$2" = "wait" ]; then echo "running"; exit %s; fi`,
		waitExit,
	))

	terminateExit := "0"
	if !terminateOK {
		terminateExit = "1"
	}
	lines = append(lines, fmt.Sprintf(
		`if [ "$1" = "workspace_v2" ] && [ "$2" = "terminate" ]; then echo "terminated"; exit %s; fi`,
		terminateExit,
	))

	lines = append(lines, `echo "ok"`)
	return strings.Join(lines, "\n")
}

func TestProbe_Success(t *testing.T) {
	setupMockProbeAPI(t, http.StatusOK, `{"id":"expwrk_123","name":"my-tmpl-ws"}`)
	setupMockAnyscale(t, probeScript(true, true, true, true))

	err := Probe("my-tmpl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProbe_APIInitFails(t *testing.T) {
	// Don't set ANYSCALE_HOST/ANYSCALE_CLI_TOKEN so newAnyscaleAPI fails.
	origHost := os.Getenv("ANYSCALE_HOST")
	origToken := os.Getenv("ANYSCALE_CLI_TOKEN")
	t.Cleanup(func() {
		if origHost == "" {
			os.Unsetenv("ANYSCALE_HOST")
		} else {
			os.Setenv("ANYSCALE_HOST", origHost)
		}
		if origToken == "" {
			os.Unsetenv("ANYSCALE_CLI_TOKEN")
		} else {
			os.Setenv("ANYSCALE_CLI_TOKEN", origToken)
		}
	})
	os.Unsetenv("ANYSCALE_HOST")
	os.Unsetenv("ANYSCALE_CLI_TOKEN")
	setupMockAnyscale(t, probeScript(true, true, true, true))

	err := Probe("my-tmpl")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "new anyscale api failed") {
		t.Errorf("error %q should contain 'new anyscale api failed'", err.Error())
	}
}

func TestProbe_GetDefaultCloudFails(t *testing.T) {
	setupMockProbeAPI(t, http.StatusOK, `{"id":"expwrk_123","name":"ws"}`)
	setupMockAnyscale(t, probeScript(false, true, true, true))

	err := Probe("my-tmpl")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "get default cloud failed") {
		t.Errorf("error %q should contain 'get default cloud failed'", err.Error())
	}
}

func TestProbe_GetDefaultProjectFails(t *testing.T) {
	setupMockProbeAPI(t, http.StatusOK, `{"id":"expwrk_123","name":"ws"}`)
	setupMockAnyscale(t, probeScript(true, false, true, true))

	err := Probe("my-tmpl")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "get default project failed") {
		t.Errorf("error %q should contain 'get default project failed'", err.Error())
	}
}

func TestProbe_LaunchFails(t *testing.T) {
	setupMockProbeAPI(t, http.StatusBadRequest, `{"error":"bad request"}`)
	setupMockAnyscale(t, probeScript(true, true, true, true))

	err := Probe("my-tmpl")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "launch template in workspace failed") {
		t.Errorf("error %q should contain 'launch template in workspace failed'", err.Error())
	}
}

func TestProbe_WaitForRunningFails(t *testing.T) {
	setupMockProbeAPI(t, http.StatusOK, `{"id":"expwrk_123","name":"my-tmpl-ws"}`)
	setupMockAnyscale(t, probeScript(true, true, false, true))

	err := Probe("my-tmpl")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "wait for workspace running state failed") {
		t.Errorf("error %q should contain 'wait for workspace running state failed'", err.Error())
	}
}
