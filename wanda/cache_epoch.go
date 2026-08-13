package wanda

import (
	"fmt"
	"time"
)

var sfoAround = time.FixedZone("SFO", -7*60*60)

// epochGroups splits the week into 2-day groups (Saturday is its own
// group) to prevent exceeding ECR's 1000 tag limit per image.
var epochGroups = [7]string{
	time.Sunday:    "a",
	time.Monday:    "a",
	time.Tuesday:   "b",
	time.Wednesday: "b",
	time.Thursday:  "c",
	time.Friday:    "c",
	time.Saturday:  "d",
}

func defaultCacheEpoch(nowFunc func() time.Time) string {
	now := nowFunc().In(sfoAround)
	group := epochGroups[now.Weekday()]

	year, week := now.Add(24 * time.Hour).ISOWeek()
	return fmt.Sprintf("%d%02d%s", year, week, group)
}

// DefaultCacheEpoch returns the default cache epoch.
func DefaultCacheEpoch() string {
	return defaultCacheEpoch(time.Now)
}
