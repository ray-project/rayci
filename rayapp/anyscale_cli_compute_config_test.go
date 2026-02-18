package rayapp

import (
	"os"
	"strings"
	"testing"
)

func TestCreateComputeConfig(t *testing.T) {
	t.Run("creates when config does not exist", func(t *testing.T) {
		script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then
    echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'
    exit 0
fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then
    echo "created compute config: $@"
    exit 0
fi
exit 1
`
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, script)}

		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString("head_node:\n  instance_type: m5.xlarge\n")
		tmpFile.Close()

		err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err != nil {
			t.Errorf("CreateComputeConfig() error = %v", err)
		}
	})

	t.Run("skips creation when config exists", func(t *testing.T) {
		script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then
    echo '{"results": [{"id": "cpt_1", "name": "my-config", "cloud_id": "cld_1", "version": 1, "created_at": "", "last_modified_at": "", "url": ""}], "metadata": {"count": 1, "next_token": null}}'
    exit 0
fi
exit 1
`
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, script)}

		err := cli.CreateComputeConfig("my-config", "/path/to/config.yaml")
		if err != nil {
			t.Errorf(
				"CreateComputeConfig() error = %v (should skip with no error when config exists)",
				err,
			)
		}
	})

	t.Run("old format with existing cloud key skips temp file", func(t *testing.T) {
		// Script has no handler for "cloud get-default", so if GetDefaultCloud
		// were called the script would exit 1, failing the test.
		script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then
    echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'
    exit 0
fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then
    echo "created compute config: $@"
    exit 0
fi
exit 1
`
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, script)}

		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		// Legacy format (head_node_type) with cloud key already set.
		tmpFile.WriteString("cloud: my-cloud\nhead_node_type:\n  name: head\n  instance_type: m5.large\n")
		tmpFile.Close()

		err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err != nil {
			t.Errorf("CreateComputeConfig() error = %v", err)
		}
	})

	t.Run("failure when create fails", func(t *testing.T) {
		script := `#!/bin/sh
if [ "$1" = "compute-config" ] && [ "$2" = "list" ]; then
    echo '{"results": [], "metadata": {"count": 0, "next_token": null}}'
    exit 0
fi
if [ "$1" = "compute-config" ] && [ "$2" = "create" ]; then
    exit 1
fi
exit 1
`
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, script)}

		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString("head_node:\n  instance_type: m5.xlarge\n")
		tmpFile.Close()

		err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err == nil {
			t.Fatal("CreateComputeConfig() error = nil, want create compute config failed")
		}
		if !strings.Contains(err.Error(), "create compute config failed") {
			t.Errorf(
				"CreateComputeConfig() error = %q, want containing 'create compute config failed'",
				err.Error(),
			)
		}
	})
}

func TestListComputeConfigs(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name       string
		script     string
		filterName *string
		wantLen    int
		wantErrStr string
		wantID     string
		wantName   string
	}{
		{
			name: "success with items",
			script: strings.Join([]string{
				"#!/bin/sh",
				`echo '{"results": [{"id": "cpt_1", "name": "my-config", "cloud_id": "cld_1", "version": 1, "created_at": "2024-01-01", "last_modified_at": "2024-01-02", "url": "https://example.com"}]}'`,
			}, "\n"),
			wantLen:  1,
			wantID:   "cpt_1",
			wantName: "my-config",
		},
		{
			name: "success with name filter",
			script: strings.Join([]string{
				"#!/bin/sh",
				`echo '{"results": [{"id": "cpt_2", "name": "filtered-config"}]}'`,
			}, "\n"),
			filterName: strPtr("filtered-config"),
			wantLen:    1,
			wantID:     "cpt_2",
			wantName:   "filtered-config",
		},
		{
			name: "empty results array",
			script: strings.Join([]string{
				"#!/bin/sh",
				`echo '{"results": []}'`,
			}, "\n"),
			wantLen: 0,
		},
		{
			name: "null results",
			script: strings.Join([]string{
				"#!/bin/sh",
				`echo '{"results": null}'`,
			}, "\n"),
			wantLen: 0,
		},
		{
			name: "missing results key",
			script: strings.Join([]string{
				"#!/bin/sh",
				`echo '{}'`,
			}, "\n"),
			wantLen: 0,
		},
		{
			name: "non-array results",
			script: strings.Join([]string{
				"#!/bin/sh",
				`echo '{"results": "not-an-array"}'`,
			}, "\n"),
			wantErrStr: "results is not an array",
		},
		{
			name: "non-object element in results",
			script: strings.Join([]string{
				"#!/bin/sh",
				`echo '{"results": ["not-an-object"]}'`,
			}, "\n"),
			wantErrStr: "results[0] is not an object",
		},
		{
			name: "item missing id",
			script: strings.Join([]string{
				"#!/bin/sh",
				`echo '{"results": [{"name": "no-id"}]}'`,
			}, "\n"),
			wantErrStr: `missing or non-string field "id"`,
		},
		{
			name: "item missing name",
			script: strings.Join([]string{
				"#!/bin/sh",
				`echo '{"results": [{"id": "cpt_1"}]}'`,
			}, "\n"),
			wantErrStr: `missing or non-string field "name"`,
		},
		{
			name:       "CLI failure",
			script:     "#!/bin/sh\nexit 1",
			wantErrStr: "list compute configs failed",
		},
		{
			name: "invalid JSON output",
			script: strings.Join([]string{
				"#!/bin/sh",
				"echo 'not valid json'",
			}, "\n"),
			wantErrStr: "parse list output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, tt.script)}

			got, err := cli.ListComputeConfigs(tt.filterName)

			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrStr) {
					t.Errorf("ListComputeConfigs() error = %q, want containing %q", err.Error(), tt.wantErrStr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("ListComputeConfigs() len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantID != "" && (len(got) == 0 || got[0].ID != tt.wantID) {
				t.Errorf("ListComputeConfigs()[0].ID = %q, want %q", got[0].ID, tt.wantID)
			}
			if tt.wantName != "" && (len(got) == 0 || got[0].Name != tt.wantName) {
				t.Errorf("ListComputeConfigs()[0].Name = %q, want %q", got[0].Name, tt.wantName)
			}
		})
	}
}

func TestGetComputeConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cli := &AnyscaleCLI{
			bin: writeFakeAnyscale(
				t,
				"#!/bin/sh\necho \"name: my-config\nhead_node:\n  instance_type: m5.xlarge\"",
			),
		}

		output, err := cli.GetComputeConfig("my-config")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "name: my-config") {
			t.Errorf("output %q should contain 'name: my-config'", output)
		}
	})

	t.Run("success with version", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\necho \"args: $@\"")}

		output, err := cli.GetComputeConfig("my-config:2")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(output, "-n my-config:2") {
			t.Errorf("output %q should contain '-n my-config:2'", output)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := &AnyscaleCLI{bin: writeFakeAnyscale(t, "#!/bin/sh\nexit 1")}

		_, err := cli.GetComputeConfig("nonexistent-config")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "get compute config failed") {
			t.Errorf("error %q should contain 'get compute config failed'", err.Error())
		}
	})
}
