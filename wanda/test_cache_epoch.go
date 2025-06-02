package wanda

import (
	"testing"
	"time"
)

func TestDefaultCacheEpoch(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		expected string
	}{{
		name:     "sunday",
		now:      time.Date(2025, 6, 1, 8, 0, 0, 0, sfoAround), // Sunday
		expected: "202522",
	}, {
		name:     "monday",
		now:      time.Date(2023, 6, 2, 12, 0, 0, 0, sfoAround), // Monday
		expected: "202522",
	}, {
		name:     "year boundary",
		now:      time.Date(2023, 12, 31, 12, 0, 0, 0, sfoAround), // Sunday
		expected: "202402",                                        // First full week of 2024
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nowFunc := func() time.Time {
				return test.now
			}

			got := defaultCacheEpoch(nowFunc)
			if got != test.expected {
				t.Errorf("DefaultCacheEpoch() = %v, want %v", got, test.expected)
			}
		})
	}
}
