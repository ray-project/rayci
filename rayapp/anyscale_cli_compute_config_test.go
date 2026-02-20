package rayapp

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestCreateComputeConfig(t *testing.T) {
	t.Run("creates when config does not exist", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString(
			strings.Join([]string{
				"head_node:", "  instance_type: m5.xlarge", "",
			}, "\n"),
		)
		tmpFile.Close()

		fake := &fakeAnyscale{}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			if len(args) >= 2 {
				switch args[0] + " " + args[1] {
				case "compute-config list":
					checkArgs(t, args,
						[]string{"compute-config", "list"},
						[]string{"--json"},
						[][2]string{{"--name", "my-config"}},
					)
				case "compute-config create":
					checkArgs(t, args,
						[]string{"compute-config", "create"},
						nil,
						[][2]string{
							{"-n", "my-config"},
							{"-f", tmpFile.Name()},
						},
					)
				}
			}
			return fake.run(args)
		})

		err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err != nil {
			t.Errorf("CreateComputeConfig() error = %v", err)
		}
	})

	t.Run("skips creation when config exists", func(t *testing.T) {
		fake := &fakeAnyscale{
			computeConfigs: []*fakeComputeConfig{
				{
					ID: "cpt_1", Name: "my-config",
					CloudID: "cld_1", Version: 1,
				},
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			if len(args) >= 2 &&
				args[0] == "compute-config" &&
				args[1] == "list" {
				checkArgs(t, args,
					[]string{"compute-config", "list"},
					[]string{"--json"},
					[][2]string{{"--name", "my-config"}},
				)
			}
			return fake.run(args)
		})

		err := cli.CreateComputeConfig(
			"my-config", "/path/to/config.yaml",
		)
		if err != nil {
			t.Errorf(
				"CreateComputeConfig() error = %v"+
					" (should skip when config exists)",
				err,
			)
		}
	})

	t.Run("old format with existing cloud key", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		// Legacy format (head_node_type) with cloud key already set.
		tmpFile.WriteString(strings.Join([]string{
			"cloud: my-cloud", "head_node_type:",
			"  name: head", "  instance_type: m5.large", "",
		}, "\n"))
		tmpFile.Close()

		fake := &fakeAnyscale{}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			if len(args) >= 2 {
				switch args[0] + " " + args[1] {
				case "compute-config list":
					checkArgs(t, args,
						[]string{"compute-config", "list"},
						[]string{"--json"},
						[][2]string{{"--name", "my-config"}},
					)
				case "compute-config create":
					pairs := [][2]string{{"-n", "my-config"}}
					checkArgs(t, args,
						[]string{"compute-config", "create"},
						nil,
						pairs,
					)
					positional := findPositionalArgs(
						args[2:], nil, pairs,
					)
					want := []string{tmpFile.Name()}
					if !slices.Equal(positional, want) {
						t.Errorf(
							"positional args = %v, want %v",
							positional, want,
						)
					}
				}
			}
			return fake.run(args)
		})

		err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err != nil {
			t.Errorf("CreateComputeConfig() error = %v", err)
		}
	})

	t.Run("old format without cloud key", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		// Legacy format (head_node_type) without cloud key.
		tmpFile.WriteString(strings.Join([]string{
			"head_node_type:",
			"  name: head", "  instance_type: m5.large", "",
		}, "\n"))
		tmpFile.Close()

		fake := &fakeAnyscale{
			defaultCloud: &fakeCloud{
				Name: "test-cloud", ID: "cld_test123",
			},
			onCreateComputeConfig: func(
				args []string,
			) (string, error) {
				// Extract positional config path from args.
				pairs := [][2]string{{"-n", "my-config"}}
				positional := findPositionalArgs(
					args[2:], nil, pairs,
				)
				if len(positional) != 1 {
					return "", fmt.Errorf(
						"fake: expected 1 positional arg,"+
							" got %v in args %v",
						positional, args,
					)
				}
				data, err := os.ReadFile(positional[0])
				if err != nil {
					return "", fmt.Errorf(
						"fake: read config: %w", err,
					)
				}
				if !strings.Contains(
					string(data), "cloud: test-cloud",
				) {
					return "", fmt.Errorf(
						"fake: expected cloud key in config,"+
							" got:\n%s", data,
					)
				}
				return "created compute config", nil
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			if len(args) >= 2 {
				switch args[0] + " " + args[1] {
				case "compute-config list":
					checkArgs(t, args,
						[]string{"compute-config", "list"},
						[]string{"--json"},
						[][2]string{{"--name", "my-config"}},
					)
				case "cloud get-default":
					checkArgs(t, args,
						[]string{"cloud", "get-default"},
						nil, nil,
					)
				case "compute-config create":
					pairs := [][2]string{{"-n", "my-config"}}
					checkArgs(t, args,
						[]string{"compute-config", "create"},
						nil,
						pairs,
					)
				}
			}
			return fake.run(args)
		})

		err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err != nil {
			t.Errorf("CreateComputeConfig() error = %v", err)
		}
	})

	t.Run("failure when create fails", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString(
			strings.Join([]string{
				"head_node:", "  instance_type: m5.xlarge", "",
			}, "\n"),
		)
		tmpFile.Close()

		fake := &fakeAnyscale{
			onCreateComputeConfig: func(
				args []string,
			) (string, error) {
				return "", fmt.Errorf(
					"anyscale error: exit status 1",
				)
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			if len(args) >= 2 {
				switch args[0] + " " + args[1] {
				case "compute-config list":
					checkArgs(t, args,
						[]string{"compute-config", "list"},
						[]string{"--json"},
						[][2]string{{"--name", "my-config"}},
					)
				case "compute-config create":
					checkArgs(t, args,
						[]string{"compute-config", "create"},
						nil,
						[][2]string{
							{"-n", "my-config"},
							{"-f", tmpFile.Name()},
						},
					)
				}
			}
			return fake.run(args)
		})

		err = cli.CreateComputeConfig("my-config", tmpFile.Name())
		if err == nil {
			t.Fatal(
				"CreateComputeConfig() error = nil," +
					" want create compute config failed",
			)
		}
		if !strings.Contains(
			err.Error(), "create compute config failed",
		) {
			t.Errorf(
				"CreateComputeConfig() error = %q,"+
					" want containing"+
					" 'create compute config failed'",
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
		wantCmd    []string
		wantFlags  []string
		wantPairs  [][2]string
		wantLen    int
		wantErrStr string
		wantID     string
		wantName   string
	}{
		{
			name: "success with items",
			runFunc: (&fakeAnyscale{
				computeConfigs: []*fakeComputeConfig{{
					ID: "cpt_1", Name: "my-config",
					CloudID: "cld_1", Version: 1,
					CreatedAt:      "2024-01-01",
					LastModifiedAt: "2024-01-02",
					URL:            "https://example.com",
				}},
			}).run,
			wantCmd:   []string{"compute-config", "list"},
			wantFlags: []string{"--json"},
			wantLen:   1,
			wantID:    "cpt_1",
			wantName:  "my-config",
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
			wantCmd:    []string{"compute-config", "list"},
			wantFlags:  []string{"--json"},
			wantPairs: [][2]string{
				{"--name", "filtered-config"},
			},
			wantLen:  1,
			wantID:   "cpt_2",
			wantName: "filtered-config",
		},
		{
			name:      "empty results",
			runFunc:   (&fakeAnyscale{}).run,
			wantCmd:   []string{"compute-config", "list"},
			wantFlags: []string{"--json"},
			wantLen:   0,
		},
		{
			name: "null results",
			runFunc: func(args []string) (string, error) {
				return `{"results": null}`, nil
			},
			wantCmd:   []string{"compute-config", "list"},
			wantFlags: []string{"--json"},
			wantLen:   0,
		},
		{
			name: "missing results key",
			runFunc: func(args []string) (string, error) {
				return `{}`, nil
			},
			wantCmd:   []string{"compute-config", "list"},
			wantFlags: []string{"--json"},
			wantLen:   0,
		},
		{
			name: "non-array results",
			runFunc: func(args []string) (string, error) {
				return `{"results": "not-an-array"}`, nil
			},
			wantCmd:    []string{"compute-config", "list"},
			wantFlags:  []string{"--json"},
			wantErrStr: "results is not an array",
		},
		{
			name: "non-object element in results",
			runFunc: func(args []string) (string, error) {
				return `{"results": ["not-an-object"]}`, nil
			},
			wantCmd:    []string{"compute-config", "list"},
			wantFlags:  []string{"--json"},
			wantErrStr: "results[0] is not an object",
		},
		{
			name: "item missing id",
			runFunc: func(args []string) (string, error) {
				return `{"results": [{"name": "no-id"}]}`, nil
			},
			wantCmd:    []string{"compute-config", "list"},
			wantFlags:  []string{"--json"},
			wantErrStr: `missing or non-string field "id"`,
		},
		{
			name: "item missing name",
			runFunc: func(args []string) (string, error) {
				return `{"results": [{"id": "cpt_1"}]}`, nil
			},
			wantCmd:    []string{"compute-config", "list"},
			wantFlags:  []string{"--json"},
			wantErrStr: `missing or non-string field "name"`,
		},
		{
			name: "CLI failure",
			runFunc: func(args []string) (string, error) {
				return "", fmt.Errorf("exit status 1")
			},
			wantCmd:    []string{"compute-config", "list"},
			wantFlags:  []string{"--json"},
			wantErrStr: "list compute configs failed",
		},
		{
			name: "invalid JSON output",
			runFunc: func(args []string) (string, error) {
				return "not valid json", nil
			},
			wantCmd:    []string{"compute-config", "list"},
			wantFlags:  []string{"--json"},
			wantErrStr: "parse list output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := NewAnyscaleCLI()
			cli.setRunFunc(func(args []string) (string, error) {
				checkArgs(
					t, args,
					tt.wantCmd, tt.wantFlags, tt.wantPairs,
				)
				return tt.runFunc(args)
			})

			got, err := cli.ListComputeConfigs(tt.filterName)

			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(
					err.Error(), tt.wantErrStr,
				) {
					t.Errorf(
						"ListComputeConfigs() error"+
							" = %q, want containing %q",
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
				t.Errorf(
					"ListComputeConfigs() len = %d, want %d",
					len(got), tt.wantLen,
				)
			}
			if tt.wantID != "" &&
				(len(got) == 0 || got[0].ID != tt.wantID) {
				t.Errorf(
					"ListComputeConfigs()[0].ID = %q, want %q",
					got[0].ID, tt.wantID,
				)
			}
			if tt.wantName != "" &&
				(len(got) == 0 || got[0].Name != tt.wantName) {
				t.Errorf(
					"ListComputeConfigs()[0].Name"+
						" = %q, want %q",
					got[0].Name, tt.wantName,
				)
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
						"name": "my-config",
						"head_node": map[string]any{
							"instance_type": "m5.xlarge",
						},
					},
				},
				{
					Name: "my-config:2",
					Config: map[string]any{
						"name": "my-config-versioned",
					},
				},
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"compute-config", "get"},
				nil, nil,
			)
			return fake.run(args)
		})

		// Test without version
		config, err := cli.GetComputeConfig("my-config")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name, _ := config["name"].(string); name != "my-config" {
			t.Errorf(
				`config["name"] = %q, want %q`,
				name, "my-config",
			)
		}

		// Test with version
		config, err = cli.GetComputeConfig("my-config:2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name, _ := config["name"].(string); name != "my-config-versioned" {
			t.Errorf(
				`config["name"] = %q, want %q`,
				name, "my-config-versioned",
			)
		}
	})

	t.Run("yaml type mismatch", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"compute-config", "get"},
				nil,
				[][2]string{{"-n", "my-config"}},
			)
			return "invalid-yaml", nil
		})

		_, err := cli.GetComputeConfig("my-config")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(
			err.Error(),
			"failed to parse compute config yaml",
		) {
			t.Errorf(
				"error %q should contain"+
					" 'failed to parse compute config yaml'",
				err.Error(),
			)
		}
	})

	t.Run("failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"compute-config", "get"},
				nil,
				[][2]string{{"-n", "nonexistent-config"}},
			)
			return "", fmt.Errorf("exit status 1")
		})

		_, err := cli.GetComputeConfig("nonexistent-config")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(
			err.Error(), "get compute config failed",
		) {
			t.Errorf(
				"error %q should contain"+
					" 'get compute config failed'",
				err.Error(),
			)
		}
	})
}
