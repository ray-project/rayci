package rayapp

import (
	"errors"
	"io"
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
		{
			name:           "no parent directory",
			configPath:     "aws.yaml",
			wantConfigName: "aws",
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

func TestConvertComputeConfig(t *testing.T) {
	dir := t.TempDir()

	oldFormat := strings.Join([]string{
		"head_node_type:",
		"  name: head",
		"  instance_type: m5.xlarge",
		"worker_node_types:",
		"  - name: worker",
		"    instance_type: m5.2xlarge",
	}, "\n")

	t.Run("success", func(t *testing.T) {
		path := filepath.Join(dir, "old.yaml")
		if err := os.WriteFile(path, []byte(oldFormat), 0644); err != nil {
			t.Fatal(err)
		}
		got, err := ConvertComputeConfig(path)
		if err != nil {
			t.Fatalf("ConvertComputeConfig() error = %v", err)
		}
		if !strings.Contains(string(got), "head_node:") {
			t.Errorf("output should contain head_node, got %s", got)
		}
		if !strings.Contains(string(got), "instance_type: m5.xlarge") {
			t.Errorf("output should contain instance_type from head, got %s", got)
		}
		if !strings.Contains(string(got), "auto_select_worker_config: true") {
			t.Errorf("output should contain auto_select_worker_config, got %s", got)
		}
	})

	t.Run("read error", func(t *testing.T) {
		_, err := ConvertComputeConfig(filepath.Join(dir, "nonexistent.yaml"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to read old config file") {
			t.Errorf("error %q should contain 'failed to read old config file'", err.Error())
		}
	})

	t.Run("parse error", func(t *testing.T) {
		path := filepath.Join(dir, "bad.yaml")
		if err := os.WriteFile(path, []byte("not: valid: yaml: ["), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := ConvertComputeConfig(path)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse old config") {
			t.Errorf("error %q should contain 'failed to parse old config'", err.Error())
		}
	})

	t.Run("marshal error", func(t *testing.T) {
		path := filepath.Join(dir, "old.yaml")
		oldFormat := strings.Join([]string{
			"head_node_type:",
			"  name: head",
			"  instance_type: m5.xlarge",
		}, "\n")
		if err := os.WriteFile(path, []byte(oldFormat), 0644); err != nil {
			t.Fatal(err)
		}
		orig := marshalNewConfig
		marshalNewConfig = func(*NewComputeConfig) ([]byte, error) {
			return nil, errors.New("marshal fail")
		}
		t.Cleanup(func() { marshalNewConfig = orig })
		_, err := ConvertComputeConfig(path)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to marshal new config") {
			t.Errorf("error %q should contain 'failed to marshal new config'", err.Error())
		}
		if !strings.Contains(err.Error(), "marshal fail") {
			t.Errorf("error %q should contain wrapped cause 'marshal fail'", err.Error())
		}
	})
}

func TestConvertComputeConfigFile(t *testing.T) {
	dir := t.TempDir()
	oldFormat := strings.Join([]string{
		"head_node_type:",
		"  name: head",
		"  instance_type: m5.large",
	}, "\n")
	oldPath := filepath.Join(dir, "old.yaml")
	if err := os.WriteFile(oldPath, []byte(oldFormat), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("success with output file", func(t *testing.T) {
		outPath := filepath.Join(dir, "new.yaml")
		err := ConvertComputeConfigFile(oldPath, outPath)
		if err != nil {
			t.Fatalf("ConvertComputeConfigFile() error = %v", err)
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read output file: %v", err)
		}
		if !strings.Contains(string(data), "head_node:") {
			t.Errorf("output file should contain head_node, got %s", data)
		}
		if !strings.Contains(string(data), "instance_type: m5.large") {
			t.Errorf("output file should contain instance_type, got %s", data)
		}
	})

	t.Run("success with empty output path writes to stdout", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stdout
		os.Stdout = w
		t.Cleanup(func() { os.Stdout = orig })
		err = ConvertComputeConfigFile(oldPath, "")
		if err != nil {
			w.Close()
			t.Fatalf("ConvertComputeConfigFile() error = %v", err)
		}
		w.Close()
		out, _ := io.ReadAll(r)
		r.Close()
		if !strings.Contains(string(out), "head_node:") {
			t.Errorf("stdout should contain head_node, got %q", out)
		}
		if !strings.Contains(string(out), "auto_select_worker_config: true") {
			t.Errorf("stdout should contain auto_select_worker_config, got %q", out)
		}
	})

	t.Run("convert error", func(t *testing.T) {
		err := ConvertComputeConfigFile(filepath.Join(dir, "nonexistent.yaml"), filepath.Join(dir, "out.yaml"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to convert compute config") {
			t.Errorf("error %q should contain 'failed to convert compute config'", err.Error())
		}
	})

	t.Run("write error", func(t *testing.T) {
		outPath := filepath.Join(dir, "subdir", "new.yaml")
		err := ConvertComputeConfigFile(oldPath, outPath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to write new config file") {
			t.Errorf("error %q should contain 'failed to write new config file'", err.Error())
		}
	})
}
