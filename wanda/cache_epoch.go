package wanda

import (
	"fmt"
	"time"
)

var sfoAround = time.FixedZone("SFO", -7*60*60)

func defaultCacheEpoch(nowFunc func() time.Time) string {
	now := nowFunc().In(sfoAround)
	var group string
	if now.Weekday() < time.Thursday {
		group = "A"
	} else {
		group = "B"
	}

	year, week := now.Add(24 * time.Hour).ISOWeek()
	return fmt.Sprintf("%d%02d%s", year, week, group)
}

// DefaultCacheEpoch returns the default cache epoch.
func DefaultCacheEpoch() string {
	return defaultCacheEpoch(time.Now)
}
