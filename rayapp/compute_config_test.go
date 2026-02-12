package rayapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateComputeConfigName(t *testing.T) {
	tests := []struct {
		name           string
		configPath     string
		wantConfigName string
	}{
		{
			name:           "basic-single-node config",
			configPath:     "configs/basic-single-node/aws.yaml",
			wantConfigName: "basic-single-node-aws",
		},
		{
			name:           "simple configs directory",
			configPath:     "configs/aws.yaml",
			wantConfigName: "configs-aws",
		},
		{
			name:           "nested directory",
			configPath:     "configs/compute/production/aws.yaml",
			wantConfigName: "production-aws",
		},
		{
			name:           "gcp config",
			configPath:     "configs/basic-single-node/gcp.yaml",
			wantConfigName: "basic-single-node-gcp",
		},
		{
			name:           "yaml extension",
			configPath:     "configs/my-config/aws.yaml",
			wantConfigName: "my-config-aws",
		},
		{
			name:           "no parent directory",
			configPath:     "aws.yaml",
			wantConfigName: "aws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateComputeConfigName(tt.configPath)
			if got != tt.wantConfigName {
				t.Errorf("generateComputeConfigName(%q) = %q, want %q", tt.configPath, got, tt.wantConfigName)
			}
		})
	}
}

func TestIsLegacyComputeConfigFormat(t *testing.T) {
	dir := t.TempDir()

	oldFormatWithHead := strings.Join([]string{
		"head_node_type:",
		"  name: head",
		"  instance_type: m5.large",
	}, "\n")
	oldFormatWithWorkers := strings.Join([]string{
		"worker_node_types:",
		"  - name: worker",
		"    instance_type: m5.xlarge",
	}, "\n")
	oldFormatBoth := strings.Join([]string{
		"head_node_type:",
		"  name: head",
		"  instance_type: m5.large",
		"worker_node_types:",
		"  - name: worker",
		"    instance_type: m5.xlarge",
	}, "\n")
	newFormat := strings.Join([]string{
		"head_node:",
		"  instance_type: m5.large",
		"auto_select_worker_config: true",
	}, "\n")

	tests := []struct {
		name     string
		content  string
		want     bool
		wantErr  bool
		setupErr bool
	}{
		{
			name:    "old format with head_node_type",
			content: oldFormatWithHead,
			want:    true,
		},
		{
			name:    "old format with worker_node_types",
			content: oldFormatWithWorkers,
			want:    true,
		},
		{
			name:    "old format with both keys",
			content: oldFormatBoth,
			want:    true,
		},
		{
			name:    "new format",
			content: newFormat,
			want:    false,
		},
		{
			name:    "comment version=legacy",
			content: "# version=legacy\n" + newFormat,
			want:    true,
		},
		{
			name:    "comment version = legacy with spaces",
			content: "  #  version = legacy  \n" + newFormat,
			want:    true,
		},
		{
			name:    "empty YAML",
			content: "",
			want:    false,
		},
		{
			name:     "missing file",
			content:  "",
			want:     false,
			wantErr:  true,
			setupErr: true,
		},
		{
			name:    "invalid YAML",
			content: "not: valid: yaml: [",
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.setupErr {
				path = filepath.Join(dir, "nonexistent.yaml")
			} else {
				f, err := os.CreateTemp(dir, "config-*.yaml")
				if err != nil {
					t.Fatal(err)
				}
				path = f.Name()
				if _, err := f.WriteString(tt.content); err != nil {
					f.Close()
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			}

			got, err := isLegacyComputeConfigFormat(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("isLegacyComputeConfigFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isLegacyComputeConfigFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasCloudKey(t *testing.T) {
	dir := t.TempDir()

	t.Run("returns true when cloud key exists", func(t *testing.T) {
		configContent := strings.Join([]string{
			"cloud: my-cloud",
			"head_node:",
			"  instance_type: m5.xlarge",
		}, "\n")
		path := filepath.Join(dir, "with-cloud.yaml")
		if err := os.WriteFile(path, []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		got, err := hasCloudKey(path)
		if err != nil {
			t.Fatalf("hasCloudKey() error = %v", err)
		}
		if got != true {
			t.Errorf("hasCloudKey() = %v, want %v", got, true)
		}
	})

	t.Run("returns false when cloud key does not exist", func(t *testing.T) {
		configContent := strings.Join([]string{
			"head_node:",
			"  instance_type: m5.xlarge",
			"auto_select_worker_config: true",
		}, "\n")
		path := filepath.Join(dir, "without-cloud.yaml")
		if err := os.WriteFile(path, []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		got, err := hasCloudKey(path)
		if err != nil {
			t.Fatalf("hasCloudKey() error = %v", err)
		}
		if got != false {
			t.Errorf("hasCloudKey() = %v, want %v", got, false)
		}
	})

	t.Run("returns false for empty YAML", func(t *testing.T) {
		path := filepath.Join(dir, "empty.yaml")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		got, err := hasCloudKey(path)
		if err != nil {
			t.Fatalf("hasCloudKey() error = %v", err)
		}
		if got != false {
			t.Errorf("hasCloudKey() = %v, want %v", got, false)
		}
	})

	t.Run("returns true when cloud key has null value", func(t *testing.T) {
		configContent := strings.Join([]string{
			"cloud: null",
			"head_node:",
			"  instance_type: m5.xlarge",
		}, "\n")
		path := filepath.Join(dir, "cloud-null.yaml")
		if err := os.WriteFile(path, []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		got, err := hasCloudKey(path)
		if err != nil {
			t.Fatalf("hasCloudKey() error = %v", err)
		}
		if got != true {
			t.Errorf("hasCloudKey() = %v, want %v (key exists even with null value)", got, true)
		}
	})

	t.Run("read error for nonexistent file", func(t *testing.T) {
		got, err := hasCloudKey(filepath.Join(dir, "nonexistent.yaml"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got != false {
			t.Errorf("hasCloudKey() = %v, want %v on error", got, false)
		}
		if !strings.Contains(err.Error(), "failed to read config file") {
			t.Errorf("error %q should contain 'failed to read config file'", err.Error())
		}
	})

	t.Run("parse error for invalid YAML", func(t *testing.T) {
		path := filepath.Join(dir, "invalid.yaml")
		if err := os.WriteFile(path, []byte("invalid: yaml: ["), 0644); err != nil {
			t.Fatal(err)
		}

		got, err := hasCloudKey(path)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got != false {
			t.Errorf("hasCloudKey() = %v, want %v on error", got, false)
		}
		if !strings.Contains(err.Error(), "failed to parse config file") {
			t.Errorf("error %q should contain 'failed to parse config file'", err.Error())
		}
	})
}

func TestAddCloudKey(t *testing.T) {
	dir := t.TempDir()

	t.Run("adds cloud key when missing", func(t *testing.T) {
		configContent := strings.Join([]string{
			"head_node:",
			"  instance_type: m5.xlarge",
			"auto_select_worker_config: true",
		}, "\n")
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		err := addCloudKey(path, "my-cloud")
		if err != nil {
			t.Fatalf("addCloudKey() error = %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read config file: %v", err)
		}
		if !strings.Contains(string(data), "cloud: my-cloud") {
			t.Errorf("config should contain 'cloud: my-cloud', got %s", data)
		}
		if !strings.Contains(string(data), "head_node:") {
			t.Errorf("config should still contain 'head_node:', got %s", data)
		}
	})

	t.Run("overwrites when cloud key exists", func(t *testing.T) {
		configContent := strings.Join([]string{
			"cloud: existing-cloud",
			"head_node:",
			"  instance_type: m5.xlarge",
		}, "\n")
		path := filepath.Join(dir, "config-with-cloud.yaml")
		if err := os.WriteFile(path, []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		err := addCloudKey(path, "new-cloud")
		if err != nil {
			t.Fatalf("addCloudKey() error = %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read config file: %v", err)
		}
		if !strings.Contains(string(data), "cloud: new-cloud") {
			t.Errorf("config should contain 'cloud: new-cloud', got %s", data)
		}
	})

	t.Run("read error", func(t *testing.T) {
		err := addCloudKey(filepath.Join(dir, "nonexistent.yaml"), "my-cloud")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to read config file") {
			t.Errorf("error %q should contain 'failed to read config file'", err.Error())
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		path := filepath.Join(dir, "invalid.yaml")
		if err := os.WriteFile(path, []byte("invalid: yaml: ["), 0644); err != nil {
			t.Fatal(err)
		}

		err := addCloudKey(path, "my-cloud")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse config file") {
			t.Errorf("error %q should contain 'failed to parse config file'", err.Error())
		}
	})

	t.Run("write error on read-only file", func(t *testing.T) {
		configContent := strings.Join([]string{
			"head_node:",
			"  instance_type: m5.xlarge",
		}, "\n")
		path := filepath.Join(dir, "readonly.yaml")
		if err := os.WriteFile(path, []byte(configContent), 0444); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(path, 0644) })

		err := addCloudKey(path, "test-cloud")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to write updated config file") {
			t.Errorf("error %q should contain 'failed to write updated config file'", err.Error())
		}
	})
}
