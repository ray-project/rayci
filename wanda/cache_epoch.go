package wanda

import (
	"fmt"
	"time"
)

var sfoAround = time.FixedZone("SFO", -7*60*60)

func defaultCacheEpoch(nowFunc func() time.Time) string {
	now := nowFunc().In(sfoAround)
	// When it is Sunday, we use the next week's epoch.
	expiry := now.Add(3 * 24 * time.Hour)
	return fmt.Sprintf("%d%03d", expiry.Year(), expiry.YearDay())
}

// DefaultCacheEpoch returns the default cache epoch.
func DefaultCacheEpoch() string {
	return defaultCacheEpoch(time.Now)
}
