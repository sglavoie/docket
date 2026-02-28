package workflow

import (
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// daysUntil returns the number of days until the given date.
// Returns negative values for past dates (overdue).
func daysUntil(t time.Time) int {
	now := time.Now()
	nowDate := util.StartOfDay(now)
	targetDate := util.StartOfDay(t)
	diff := targetDate.Sub(nowDate)
	return int(diff.Hours() / 24)
}

// daysSince returns the number of days since the given date.
// Always returns 0 or positive values.
func daysSince(t time.Time) int {
	now := time.Now()
	nowDate := util.StartOfDay(now)
	targetDate := util.StartOfDay(t)
	diff := nowDate.Sub(targetDate)
	days := int(diff.Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
