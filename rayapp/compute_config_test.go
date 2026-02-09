package rayapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseComputeConfigName(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseComputeConfigName(tt.configPath)
			if got != tt.wantConfigName {
				t.Errorf("parseComputeConfigName(%q) = %q, want %q", tt.configPath, got, tt.wantConfigName)
			}
		})
	}
}

func TestIsOldComputeConfigFormat(t *testing.T) {
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

			got, err := isOldComputeConfigFormat(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("isOldComputeConfigFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isOldComputeConfigFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}
