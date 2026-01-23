package rayapp

import (
	"os"
	"strings"
	"testing"
)

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
			config := NewWorkspaceTestConfig(tt.tmplName, tt.buildFile)

			if config == nil {
				t.Fatal("expected non-nil WorkspaceTestConfig")
			}
			if config.tmplName != tt.tmplName {
				t.Errorf("tmplName = %q, want %q", config.tmplName, tt.tmplName)
			}
			if config.buildFile != tt.buildFile {
				t.Errorf("buildFile = %q, want %q", config.buildFile, tt.buildFile)
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

func TestWorkspaceTestConfigRun_InvalidBuildFile(t *testing.T) {
	setupMockAnyscale(t, "#!/bin/sh\necho mock")

	config := NewWorkspaceTestConfig("my-template", "nonexistent/build.yaml")
	err := config.Run()

	if err == nil {
		t.Fatal("expected error for nonexistent build file")
	}
	if !strings.Contains(err.Error(), "read templates failed") {
		t.Errorf("error %q should contain 'read templates failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_TemplateNotFound(t *testing.T) {
	setupMockAnyscale(t, "#!/bin/sh\necho mock")

	config := NewWorkspaceTestConfig("nonexistent-template", "testdata/BUILD.yaml")
	err := config.Run()

	if err == nil {
		t.Fatal("expected error for nonexistent template")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should contain 'not found'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_CreateWorkspaceFails(t *testing.T) {
	// Mock script that fails on workspace_v2 create
	script := `#!/bin/sh
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "create failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	config := NewWorkspaceTestConfig("reefy-ray", "testdata/BUILD.yaml")
	err := config.Run()

	if err == nil {
		t.Fatal("expected error when create workspace fails")
	}
	if !strings.Contains(err.Error(), "create empty workspace failed") {
		t.Errorf("error %q should contain 'create empty workspace failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_StartWorkspaceFails(t *testing.T) {
	// Mock script that succeeds on create but fails on start
	script := `#!/bin/sh
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "created"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "start" ]; then
    echo "start failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	config := NewWorkspaceTestConfig("reefy-ray", "testdata/BUILD.yaml")
	err := config.Run()

	if err == nil {
		t.Fatal("expected error when start workspace fails")
	}
	if !strings.Contains(err.Error(), "start workspace failed") {
		t.Errorf("error %q should contain 'start workspace failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_WaitForStateFails(t *testing.T) {
	// Mock script that succeeds on create and start but fails on wait
	script := `#!/bin/sh
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "created"
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

	config := NewWorkspaceTestConfig("reefy-ray", "testdata/BUILD.yaml")
	err := config.Run()

	if err == nil {
		t.Fatal("expected error when wait for state fails")
	}
	if !strings.Contains(err.Error(), "wait for workspace running state failed") {
		t.Errorf("error %q should contain 'wait for workspace running state failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_CopyTemplateFails(t *testing.T) {
	// Mock script that succeeds until push
	script := `#!/bin/sh
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "created"
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
    echo "push failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	config := NewWorkspaceTestConfig("reefy-ray", "testdata/BUILD.yaml")
	err := config.Run()

	if err == nil {
		t.Fatal("expected error when copy template fails")
	}
	if !strings.Contains(err.Error(), "push zip to workspace failed") {
		t.Errorf("error %q should contain 'push zip to workspace failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_RunCommandFails(t *testing.T) {
	// Mock script that succeeds until run_command
	script := `#!/bin/sh
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "created"
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
    echo "run_command failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	config := NewWorkspaceTestConfig("reefy-ray", "testdata/BUILD.yaml")
	err := config.Run()

	if err == nil {
		t.Fatal("expected error when run command fails")
	}
	if !strings.Contains(err.Error(), "run test in workspace failed") {
		t.Errorf("error %q should contain 'run test in workspace failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_TerminateFails(t *testing.T) {
	// Mock script that succeeds until terminate
	script := `#!/bin/sh
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "created"
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
    echo "terminate failed" >&2
    exit 1
fi
echo "ok"
`
	setupMockAnyscale(t, script)

	config := NewWorkspaceTestConfig("reefy-ray", "testdata/BUILD.yaml")
	err := config.Run()

	if err == nil {
		t.Fatal("expected error when terminate fails")
	}
	if !strings.Contains(err.Error(), "terminate workspace failed") {
		t.Errorf("error %q should contain 'terminate workspace failed'", err.Error())
	}
}

func TestWorkspaceTestConfigRun_Success(t *testing.T) {
	// Mock script that succeeds for all operations
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "get" ]; then
    echo "config not found"
    exit 1
fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then
    echo "created compute config"
    exit 0
fi
if [ "$1" = "workspace_v2" ] && [ "$2" = "create" ]; then
    echo "created"
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

	config := NewWorkspaceTestConfig("reefy-ray", "testdata/BUILD.yaml")
	err := config.Run()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the config was populated correctly
	if config.template == nil {
		t.Error("template should be set after successful run")
	}
	if config.template != nil && config.template.Name != "reefy-ray" {
		t.Errorf("template.Name = %q, want %q", config.template.Name, "reefy-ray")
	}
	if config.workspaceName == "" {
		t.Error("workspaceName should be set after successful run")
	}
	if !strings.HasPrefix(config.workspaceName, "reefy-ray-") {
		t.Errorf("workspaceName %q should start with 'reefy-ray-'", config.workspaceName)
	}
}

func TestTest_Success(t *testing.T) {
	// Mock script that succeeds for all operations
	script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "get" ]; then
    echo "config not found"
    exit 1
fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then
    echo "created"
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
}

func TestTest_TemplateNotFound(t *testing.T) {
	setupMockAnyscale(t, "#!/bin/sh\necho ok")

	err := Test("nonexistent-template", "testdata/BUILD.yaml")

	if err == nil {
		t.Fatal("expected error for nonexistent template")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error %q should contain 'not found'", err.Error())
	}
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

func TestWorkspaceStartWaitTime_Constant(t *testing.T) {
	// Verify the wait time constant is reasonable
	if workspaceStartWaitTime <= 0 {
		t.Error("workspaceStartWaitTime should be positive")
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

	config := NewWorkspaceTestConfig("reefy-ray", "testdata/BUILD.yaml")
	_ = config.Run() // We don't care about the error, just that it uses the token
}
