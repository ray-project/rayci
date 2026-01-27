package goqualgate

import (
	"strings"
	"testing"
)

func TestCheckCoverage(t *testing.T) {
	tests := []struct {
		name     string
		cfg      CoverageConfig
		coverage PackageCoverage
		wantErr  bool
	}{
		{
			name:     "no threshold all pass",
			cfg:      CoverageConfig{MinCoveragePct: 0},
			coverage: PackageCoverage{"pkg": 80.0},
			wantErr:  false,
		},
		{
			name:     "above threshold",
			cfg:      CoverageConfig{MinCoveragePct: 50.0},
			coverage: PackageCoverage{"pkg": 80.0},
			wantErr:  false,
		},
		{
			name:     "exactly at threshold",
			cfg:      CoverageConfig{MinCoveragePct: 80.0},
			coverage: PackageCoverage{"pkg": 80.0},
			wantErr:  false,
		},
		{
			name:     "below threshold",
			cfg:      CoverageConfig{MinCoveragePct: 90.0},
			coverage: PackageCoverage{"pkg": 80.0},
			wantErr:  true,
		},
		{
			name:     "multiple packages one fails",
			cfg:      CoverageConfig{MinCoveragePct: 50.0},
			coverage: PackageCoverage{"pkg1": 80.0, "pkg2": 30.0},
			wantErr:  true,
		},
		{
			name:     "multiple packages all pass",
			cfg:      CoverageConfig{MinCoveragePct: 50.0},
			coverage: PackageCoverage{"pkg1": 80.0, "pkg2": 60.0},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.checkCoverage(tt.coverage)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkCoverage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "coverage check failed") {
				t.Errorf("checkCoverage() error = %v, want error containing 'coverage check failed'", err)
			}
		})
	}
}
