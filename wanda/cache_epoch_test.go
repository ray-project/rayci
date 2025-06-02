package wanda

import (
	"testing"
	"time"
)

func TestDefaultCacheEpoch(t *testing.T) {
	date := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 8, 0, 0, 0, sfoAround)
	}

	tests := []struct {
		name     string
		now      time.Time
		expected string
	}{{
		name:     "saturday",
		now:      date(2025, time.May, 31), // Saturday
		expected: "202522",
	}, {
		name:     "sunday",
		now:      date(2025, time.June, 1), // Sunday
		expected: "202523",
	}, {
		name:     "monday",
		now:      date(2025, time.June, 2), // Monday
		expected: "202523",
	}, {
		name:     "year boundary",
		now:      date(2023, time.December, 31), // Sunday
		expected: "202401",                      // First full week of 2024
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
