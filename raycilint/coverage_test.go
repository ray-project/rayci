package raycilint

import (
	"strings"
	"testing"
)

func TestCheckCoverage(t *testing.T) {
	tests := []struct {
		name     string
		pct      float64
		coverage PackageCoverage
		wantErr  bool
	}{
		{
			name:     "no threshold all pass",
			pct:      0,
			coverage: PackageCoverage{"pkg": 80.0},
			wantErr:  false,
		},
		{
			name:     "above threshold",
			pct:      50.0,
			coverage: PackageCoverage{"pkg": 80.0},
			wantErr:  false,
		},
		{
			name:     "exactly at threshold",
			pct:      80.0,
			coverage: PackageCoverage{"pkg": 80.0},
			wantErr:  false,
		},
		{
			name:     "below threshold",
			pct:      90.0,
			coverage: PackageCoverage{"pkg": 80.0},
			wantErr:  true,
		},
		{
			name:     "multiple packages one fails",
			pct:      50.0,
			coverage: PackageCoverage{"pkg1": 80.0, "pkg2": 30.0},
			wantErr:  true,
		},
		{
			name:     "multiple packages all pass",
			pct:      50.0,
			coverage: PackageCoverage{"pkg1": 80.0, "pkg2": 60.0},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newConfig()
			cfg.Coverage.MinCoveragePct = tt.pct
			err := checkCoverage(cfg, tt.coverage)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkCoverage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "coverage check failed") {
				t.Errorf("checkCoverage() error = %v, want error containing 'coverage check failed'", err)
			}
		})
	}
}
