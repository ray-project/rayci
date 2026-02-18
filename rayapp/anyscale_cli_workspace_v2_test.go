package rayapp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceStateString(t *testing.T) {
	tests := []struct {
		state WorkspaceState
		want  string
	}{
		{StateTerminated, "TERMINATED"},
		{StateStarting, "STARTING"},
		{StateRunning, "RUNNING"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("WorkspaceState.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetWorkspaceDescription(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		script := `#!/bin/sh
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"id": "expwrk_abc123", "name": "my-workspace", "state": "RUNNING"}'
    exit 0
fi
exit 1
`
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, script)}

		got, err := cli.getWorkspaceDescription("my-workspace")
		if err != nil {
			t.Fatalf("getWorkspaceDescription() error = %v", err)
		}
		if got["id"] != "expwrk_abc123" {
			t.Errorf("got[%q] = %v, want expwrk_abc123", "id", got["id"])
		}
		if got["name"] != "my-workspace" {
			t.Errorf("got[%q] = %v, want my-workspace", "name", got["name"])
		}
		if got["state"] != "RUNNING" {
			t.Errorf("got[%q] = %v, want RUNNING", "state", got["state"])
		}
	})

	t.Run("CLI failure", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\nexit 1")}

		_, err := cli.getWorkspaceDescription("my-workspace")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "get workspace failed") {
			t.Errorf("error %q should contain 'get workspace failed'", err.Error())
		}
	})

	t.Run("invalid JSON output", func(t *testing.T) {
		cli := &AnyscaleCLI{
			bin: writeFakeAnyscale(t, "#!/bin/sh\necho 'not valid json'"),
		}

		_, err := cli.getWorkspaceDescription("my-workspace")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "parse workspace get output") {
			t.Errorf("error %q should contain 'parse workspace get output'", err.Error())
		}
	})
}

func TestGetWorkspaceID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		script := `#!/bin/sh
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"id": "expwrk_xyz789", "name": "test-ws"}'
    exit 0
fi
exit 1
`
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, script)}

		got, err := cli.getWorkspaceID("test-ws")
		if err != nil {
			t.Fatalf("getWorkspaceID() error = %v", err)
		}
		if got != "expwrk_xyz789" {
			t.Errorf("getWorkspaceID() = %q, want expwrk_xyz789", got)
		}
	})

	t.Run("getWorkspaceDescription fails", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\nexit 1")}

		_, err := cli.getWorkspaceID("my-workspace")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "get workspace description failed") {
			t.Errorf("error %q should contain 'get workspace description failed'", err.Error())
		}
	})

	t.Run("id missing in description", func(t *testing.T) {
		script := `#!/bin/sh
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"name": "no-id-workspace"}'
    exit 0
fi
exit 1
`
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, script)}

		_, err := cli.getWorkspaceID("no-id-workspace")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "workspace ID not found in description") {
			t.Errorf("error %q should contain 'workspace ID not found in description'", err.Error())
		}
	})

	t.Run("id not a string", func(t *testing.T) {
		script := `#!/bin/sh
if [ "$1" = "workspace_v2" ] && [ "$2" = "get" ]; then
    echo '{"id": 12345, "name": "bad-id"}'
    exit 0
fi
exit 1
`
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, script)}

		_, err := cli.getWorkspaceID("bad-id")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "workspace ID not found in description") {
			t.Errorf("error %q should contain 'workspace ID not found in description'", err.Error())
		}
	})
}

func TestCreateEmptyWorkspace(t *testing.T) {
	tests := []struct {
		name          string
		script        string
		config        *WorkspaceTestConfig
		wantErr       bool
		errContains   string
		wantArgSubstr string
	}{
		{
			name:   "success without compute config",
			script: "#!/bin/sh\necho \"args: $@\"\necho \"(anyscale +1.0s) Workspace created successfully id: expwrk_testid123\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "anyscaleray2441-py312-cu128",
					},
				},
			},
			wantArgSubstr: "workspace_v2 create",
		},
		{
			name:   "success with compute config name",
			script: "#!/bin/sh\necho \"args: $@\"\necho \"(anyscale +1.0s) Workspace created successfully id: expwrk_testid123\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				computeConfig: "basic-single-node-aws",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "anyscaleray2441-py312-cu128",
					},
				},
			},
			wantArgSubstr: "--compute-config",
		},
		{
			name:   "success with BYOD containerfile",
			script: "#!/bin/sh\necho \"args: $@\"\necho \"(anyscale +1.0s) Workspace created successfully id: expwrk_testid123\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				buildDir:      filepath.Join("foo", "bar"),
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BYOD: &ClusterEnvBYOD{
							ContainerFile: "Dockerfile",
							RayVersion:    "2.34.0",
						},
					},
				},
			},
			wantArgSubstr: "--containerfile",
		},
		{
			name:   "success with ImageURI",
			script: "#!/bin/sh\necho \"args: $@\"\necho \"(anyscale +1.0s) Workspace created successfully id: expwrk_testid123\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						ImageURI: "anyscale/ray:2.44.1-py312-cu128",
					},
				},
			},
			wantArgSubstr: "--image-uri anyscale/ray:2.44.1-py312-cu128",
		},
		{
			name:   "success with ImageURI and compute config",
			script: "#!/bin/sh\necho \"args: $@\"\necho \"(anyscale +1.0s) Workspace created successfully id: expwrk_testid123\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				computeConfig: "basic-single-node-aws",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						ImageURI: "anyscale/ray:2.35.0-py311",
					},
				},
			},
			wantArgSubstr: "--image-uri anyscale/ray:2.35.0-py311",
		},
		{
			name:   "invalid build ID",
			script: "#!/bin/sh\necho \"args: $@\"",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "invalid-build-id",
					},
				},
			},
			wantErr:     true,
			errContains: "cluster env",
		},
		{
			name:   "CLI error",
			script: "#!/bin/sh\nexit 1",
			config: &WorkspaceTestConfig{
				workspaceName: "test-workspace",
				template: &Template{
					ClusterEnv: &ClusterEnv{
						BuildID: "anyscaleray2441-py312-cu128",
					},
				},
			},
			wantErr:     true,
			errContains: "create empty workspace failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, tt.script)}

			err := cli.createEmptyWorkspace(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPushFolderToWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\necho \"push $@\"")}

		err := cli.pushFolderToWorkspace("my-workspace", "/local/path")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\nexit 1")}

		err := cli.pushFolderToWorkspace("my-workspace", "/local/path")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "push file to workspace failed") {
			t.Errorf("error %q should contain 'push file to workspace failed'", err.Error())
		}
	})
}

func TestTerminateWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\necho \"terminating $@\"")}

		err := cli.terminateWorkspace("my-workspace")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\nexit 1")}

		err := cli.terminateWorkspace("my-workspace")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "terminate workspace failed") {
			t.Errorf("error %q should contain 'terminate workspace failed'", err.Error())
		}
	})
}

func TestRunCmdInWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\necho \"running: $@\"")}

		err := cli.runCmdInWorkspace("test-workspace", "echo hello")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\nexit 1")}

		err := cli.runCmdInWorkspace("test-workspace", "failing-command")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "run command in workspace failed") {
			t.Errorf("error %q should contain 'run command in workspace failed'", err.Error())
		}
	})
}

func TestStartWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\necho \"starting $@\"")}

		err := cli.startWorkspace("test-workspace")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\nexit 1")}

		err := cli.startWorkspace("test-workspace")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "start workspace failed") {
			t.Errorf("error %q should contain 'start workspace failed'", err.Error())
		}
	})
}

func TestGetWorkspaceStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\necho \"RUNNING\"")}

		output, err := cli.getWorkspaceStatus("test-workspace")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "RUNNING") {
			t.Errorf("output %q should contain 'RUNNING'", output)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\nexit 1")}

		_, err := cli.getWorkspaceStatus("test-workspace")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "get workspace state failed") {
			t.Errorf("error %q should contain 'get workspace state failed'", err.Error())
		}
	})
}

func TestWaitForWorkspaceState(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\necho \"state reached\"")}

		output, err := cli.waitForWorkspaceState("test-workspace", StateRunning)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "state reached") {
			t.Errorf("output %q should contain 'state reached'", output)
		}
	})

	t.Run("wait for terminated", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\necho \"terminated\"")}

		output, err := cli.waitForWorkspaceState("test-workspace", StateTerminated)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "terminated") {
			t.Errorf("output %q should contain 'terminated'", output)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\nexit 1")}

		_, err := cli.waitForWorkspaceState("test-workspace", StateRunning)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "wait for workspace state failed") {
			t.Errorf("error %q should contain 'wait for workspace state failed'", err.Error())
		}
	})
}
