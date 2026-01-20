package goqualgate

import (
	"strings"
	"testing"
)

func TestCompareCoverage(t *testing.T) {
	tests := []struct {
		name    string
		cfg     CoverageConfig
		base    PackageCoverage
		current PackageCoverage
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no change",
			cfg:     CoverageConfig{Threshold: 1.0},
			base:    PackageCoverage{"pkg": 80.0},
			current: PackageCoverage{"pkg": 80.0},
			wantErr: false,
		},
		{
			name:    "coverage improved",
			cfg:     CoverageConfig{Threshold: 1.0},
			base:    PackageCoverage{"pkg": 70.0},
			current: PackageCoverage{"pkg": 80.0},
			wantErr: false,
		},
		{
			name:    "coverage decreased within threshold",
			cfg:     CoverageConfig{Threshold: 5.0},
			base:    PackageCoverage{"pkg": 80.0},
			current: PackageCoverage{"pkg": 76.0},
			wantErr: false,
		},
		{
			name:    "coverage decreased beyond threshold",
			cfg:     CoverageConfig{Threshold: 1.0},
			base:    PackageCoverage{"pkg": 80.0},
			current: PackageCoverage{"pkg": 70.0},
			wantErr: true,
			errMsg:  "coverage check failed",
		},
		{
			name:    "new package without threshold",
			cfg:     CoverageConfig{Threshold: 1.0, NewPackageThreshold: 0},
			base:    PackageCoverage{"pkg1": 80.0},
			current: PackageCoverage{"pkg1": 80.0, "pkg2": 20.0},
			wantErr: false,
		},
		{
			name:    "new package meets threshold",
			cfg:     CoverageConfig{Threshold: 1.0, NewPackageThreshold: 50.0},
			base:    PackageCoverage{"pkg1": 80.0},
			current: PackageCoverage{"pkg1": 80.0, "pkg2": 60.0},
			wantErr: false,
		},
		{
			name:    "new package below threshold",
			cfg:     CoverageConfig{Threshold: 1.0, NewPackageThreshold: 50.0},
			base:    PackageCoverage{"pkg1": 80.0},
			current: PackageCoverage{"pkg1": 80.0, "pkg2": 30.0},
			wantErr: true,
			errMsg:  "coverage check failed",
		},
		{
			name:    "removed package is informational",
			cfg:     CoverageConfig{Threshold: 1.0},
			base:    PackageCoverage{"pkg1": 80.0, "pkg2": 90.0},
			current: PackageCoverage{"pkg1": 80.0},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := compareCoverage(tt.cfg, tt.base, tt.current)
			if (err != nil) != tt.wantErr {
				t.Errorf("compareCoverage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("compareCoverage() error = %v, want error containing %q", err, tt.errMsg)
			}
		})
	}
}

func TestRunPRMissingBaseBranch(t *testing.T) {
	cfg := CoverageConfig{
		IsPR:       true,
		BaseBranch: "",
	}

	err := cfg.Run()
	if err == nil {
		t.Error("expected error for PR without base branch")
	}
	if !strings.Contains(err.Error(), "base branch required") {
		t.Errorf("unexpected error: %v", err)
	}
}
