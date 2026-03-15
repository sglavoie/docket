package workflow

import (
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// daysUntil returns the number of calendar days until the given date.
// Returns negative values for past dates (overdue).
// Uses util.DaysBetween for DST-safe calendar-day arithmetic.
func daysUntil(t time.Time) int {
	return util.DaysBetween(time.Now(), t)
}

// daysSince returns the number of calendar days since the given date.
// Always returns 0 or positive values.
// Uses util.DaysBetween for DST-safe calendar-day arithmetic.
func daysSince(t time.Time) int {
	days := util.DaysBetween(t, time.Now())
	if days < 0 {
		return 0
	}
	return days
}
