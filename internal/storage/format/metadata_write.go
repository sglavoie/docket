package format

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// FormatRecurrence converts a RecurrencePattern to its string representation for storage.
func FormatRecurrence(r *models.RecurrencePattern) string {
	if r == nil || r.Interval <= 0 || r.Interval > models.MaxRecurrenceInterval {
		return ""
	}

	var unit string
	switch r.Unit {
	case models.RecurrenceDaily:
		unit = "d"
	case models.RecurrenceWeekly:
		unit = "w"
	case models.RecurrenceMonthly:
		unit = "m"
	default:
		return ""
	}
	return fmt.Sprintf("%d%s", r.Interval, unit)
}
