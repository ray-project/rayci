package wanda

import (
	"fmt"
	"time"
)

var sfoAround = time.FixedZone("SFO", -7*60*60)

func defaultCacheEpoch(nowFunc func() time.Time) string {
	now := nowFunc().In(sfoAround)
	// When it is Sunday, we use the next week's epoch.
	year, week := now.Add(time.Hour * 24 * 7).ISOWeek()
	return fmt.Sprintf("%d%02d", year, week)
}

// DefaultCacheEpoch returns the default cache epoch.
func DefaultCacheEpoch() string {
	return defaultCacheEpoch(time.Now)
}
