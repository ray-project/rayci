package rayapp

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestCreateComputeConfig(t *testing.T) {
	t.Run("creates when config does not exist", func(t *testing.T) {
		fake := &fakeAnyscale{}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(fake.run)

		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString(
			strings.Join([]string{"head_node:", "  instance_type: m5.xlarge", ""}, "\n"),
		)
		tmpFile.Close()

		err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err != nil {
			t.Errorf("CreateComputeConfig() error = %v", err)
		}
	})

	t.Run("skips creation when config exists", func(t *testing.T) {
		fake := &fakeAnyscale{
			computeConfigs: []*fakeComputeConfig{
				{ID: "cpt_1", Name: "my-config", CloudID: "cld_1", Version: 1},
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(fake.run)

		err := cli.CreateComputeConfig("my-config", "/path/to/config.yaml")
		if err != nil {
			t.Errorf(
				"CreateComputeConfig() error = %v (should skip with no error when config exists)",
				err,
			)
		}
	})

	t.Run("old format with existing cloud key skips temp file", func(t *testing.T) {
		fake := &fakeAnyscale{}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(fake.run)

		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		// Legacy format (head_node_type) with cloud key already set.
		tmpFile.WriteString(strings.Join([]string{
			"cloud: my-cloud", "head_node_type:", "  name: head", "  instance_type: m5.large", "",
		}, "\n"))
		tmpFile.Close()

		err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err != nil {
			t.Errorf("CreateComputeConfig() error = %v", err)
		}
	})

	t.Run("old format without cloud key fetches default cloud", func(t *testing.T) {
		fake := &fakeAnyscale{
			defaultCloud: &fakeCloud{Name: "test-cloud", ID: "cld_test123"},
			onCreateComputeConfig: func(args []string) (string, error) {
				// Old format uses positional path: create -n <name> <path>
				configPath := args[4]
				data, err := os.ReadFile(configPath)
				if err != nil {
					return "", fmt.Errorf("fake: read config: %w", err)
				}
				if !strings.Contains(string(data), "cloud: test-cloud") {
					return "", fmt.Errorf(
						"fake: expected cloud key in config, got:\n%s", data,
					)
				}
				return "created compute config", nil
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(fake.run)

		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		// Legacy format (head_node_type) without cloud key.
		tmpFile.WriteString(strings.Join([]string{
			"head_node_type:", "  name: head", "  instance_type: m5.large", "",
		}, "\n"))
		tmpFile.Close()

		err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err != nil {
			t.Errorf("CreateComputeConfig() error = %v", err)
		}
	})

	t.Run("failure when create fails", func(t *testing.T) {
		fake := &fakeAnyscale{
			onCreateComputeConfig: func(args []string) (string, error) {
				return "", fmt.Errorf("anyscale error: exit status 1")
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(fake.run)

		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString(
			strings.Join([]string{"head_node:", "  instance_type: m5.xlarge", ""}, "\n"),
		)
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
		runFunc    func(args []string) (string, error)
		filterName *string
		wantLen    int
		wantErrStr string
		wantID     string
		wantName   string
	}{
		{
			name: "success with items",
			runFunc: (&fakeAnyscale{
				computeConfigs: []*fakeComputeConfig{{
					ID: "cpt_1", Name: "my-config", CloudID: "cld_1", Version: 1,
					CreatedAt: "2024-01-01", LastModifiedAt: "2024-01-02",
					URL: "https://example.com",
				}},
			}).run,
			wantLen:  1,
			wantID:   "cpt_1",
			wantName: "my-config",
		},
		{
			name: "success with name filter",
			runFunc: (&fakeAnyscale{
				computeConfigs: []*fakeComputeConfig{
					{ID: "cpt_1", Name: "other-config"},
					{ID: "cpt_2", Name: "filtered-config"},
				},
			}).run,
			filterName: strPtr("filtered-config"),
			wantLen:    1,
			wantID:     "cpt_2",
			wantName:   "filtered-config",
		},
		{
			name:    "empty results",
			runFunc: (&fakeAnyscale{}).run,
			wantLen: 0,
		},
		{
			name: "null results",
			runFunc: func(args []string) (string, error) {
				return `{"results": null}`, nil
			},
			wantLen: 0,
		},
		{
			name: "missing results key",
			runFunc: func(args []string) (string, error) {
				return `{}`, nil
			},
			wantLen: 0,
		},
		{
			name: "non-array results",
			runFunc: func(args []string) (string, error) {
				return `{"results": "not-an-array"}`, nil
			},
			wantErrStr: "results is not an array",
		},
		{
			name: "non-object element in results",
			runFunc: func(args []string) (string, error) {
				return `{"results": ["not-an-object"]}`, nil
			},
			wantErrStr: "results[0] is not an object",
		},
		{
			name: "item missing id",
			runFunc: func(args []string) (string, error) {
				return `{"results": [{"name": "no-id"}]}`, nil
			},
			wantErrStr: `missing or non-string field "id"`,
		},
		{
			name: "item missing name",
			runFunc: func(args []string) (string, error) {
				return `{"results": [{"id": "cpt_1"}]}`, nil
			},
			wantErrStr: `missing or non-string field "name"`,
		},
		{
			name: "CLI failure",
			runFunc: func(args []string) (string, error) {
				return "", fmt.Errorf("exit status 1")
			},
			wantErrStr: "list compute configs failed",
		},
		{
			name: "invalid JSON output",
			runFunc: func(args []string) (string, error) {
				return "not valid json", nil
			},
			wantErrStr: "parse list output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := NewAnyscaleCLI()
			cli.setRunFunc(tt.runFunc)

			got, err := cli.ListComputeConfigs(tt.filterName)

			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrStr) {
					t.Errorf(
						"ListComputeConfigs() error = %q, want containing %q",
						err.Error(),
						tt.wantErrStr,
					)
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
		fake := &fakeAnyscale{
			computeConfigs: []*fakeComputeConfig{
				{
					Name: "my-config",
					Config: map[string]any{
						"name":      "my-config",
						"head_node": map[string]any{"instance_type": "m5.xlarge"},
					},
				},
				{
					Name:   "my-config:2",
					Config: map[string]any{"name": "my-config-versioned"},
				},
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(fake.run)

		// Test without version
		config, err := cli.GetComputeConfig("my-config")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name, _ := config["name"].(string); name != "my-config" {
			t.Errorf(`config["name"] = %q, want %q`, name, "my-config")
		}

		// Test with version
		config, err = cli.GetComputeConfig("my-config:2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name, _ := config["name"].(string); name != "my-config-versioned" {
			t.Errorf(`config["name"] = %q, want %q`, name, "my-config-versioned")
		}
	})

	t.Run("yaml type mismatch", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			return "invalid-yaml", nil
		})

		_, err := cli.GetComputeConfig("my-config")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse compute config yaml") {
			t.Errorf("error %q should contain 'failed to parse compute config yaml'", err.Error())
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			return "", fmt.Errorf("exit status 1")
		})

		_, err := cli.GetComputeConfig("nonexistent-config")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "get compute config failed") {
			t.Errorf("error %q should contain 'get compute config failed'", err.Error())
		}
	})
}
