package wanda

import (
	"testing"
	"time"
)

func TestDefaultCacheEpoch(t *testing.T) {
	morningOf := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 8, 0, 0, 0, sfoAround)
	}

	tests := []struct {
		name     string
		now      time.Time
		expected string
	}{{
		name:     "saturday",
		now:      morningOf(2025, time.May, 31), // Saturday
		expected: "202522B",
	}, {
		name:     "sunday",
		now:      morningOf(2025, time.June, 1), // Sunday
		expected: "202523A",
	}, {
		name:     "monday",
		now:      morningOf(2025, time.June, 2), // Monday
		expected: "202523A",
	}, {
		name:     "tuesday",
		now:      morningOf(2025, time.June, 3), // Tuesday
		expected: "202523A",
	}, {
		name:     "wednesday",
		now:      morningOf(2025, time.June, 4), // Wednesday
		expected: "202523A",
	}, {
		name:     "thursday",
		now:      morningOf(2025, time.June, 5), // Thursday
		expected: "202523B",
	}, {
		name:     "friday",
		now:      morningOf(2025, time.June, 6), // Friday
		expected: "202523B",
	}, {
		name:     "year boundary",
		now:      morningOf(2023, time.December, 31), // Sunday
		expected: "202401A",                          // First full week of 2024
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
