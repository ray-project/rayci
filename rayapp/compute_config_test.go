package rayapp

import (
	"testing"
)

func TestParseComputeConfigName(t *testing.T) {
	tests := []struct {
		name           string
		awsConfigPath  string
		wantConfigName string
	}{
		{
			name:           "basic-single-node config",
			awsConfigPath:  "configs/basic-single-node/aws.yaml",
			wantConfigName: "basic-single-node-aws",
		},
		{
			name:           "simple configs directory",
			awsConfigPath:  "configs/aws.yaml",
			wantConfigName: "configs-aws",
		},
		{
			name:           "nested directory",
			awsConfigPath:  "configs/compute/production/aws.yaml",
			wantConfigName: "production-aws",
		},
		{
			name:           "gcp config",
			awsConfigPath:  "configs/basic-single-node/gcp.yaml",
			wantConfigName: "basic-single-node-gcp",
		},
		{
			name:           "yaml extension",
			awsConfigPath:  "configs/my-config/aws.yaml",
			wantConfigName: "my-config-aws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseComputeConfigName(tt.awsConfigPath)
			if got != tt.wantConfigName {
				t.Errorf("parseComputeConfigName(%q) = %q, want %q", tt.awsConfigPath, got, tt.wantConfigName)
			}
		})
	}
}
