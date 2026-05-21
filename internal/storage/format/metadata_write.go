package format

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// WriteMetadata generates the metadata comment for a task.
func WriteMetadata(task *models.Task) string {
	var parts []string
	metadataFieldRegistry.appendWrittenParts(task, &parts)
	return fmt.Sprintf("<!-- %s -->", strings.Join(parts, ", "))
}

// FormatRecurrence converts a RecurrencePattern to its string representation for storage.
func FormatRecurrence(r *models.RecurrencePattern) string {
	if r == nil {
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
	case models.RecurrenceYearly:
		unit = "y"
	default:
		unit = "d"
	}

	base := fmt.Sprintf("%d%s", r.Interval, unit)

	if r.Unit == models.RecurrenceWeekly && len(r.Weekdays) > 0 {
		dayNames := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
		var days []string
		for _, d := range r.Weekdays {
			if d >= 0 && d < 7 {
				days = append(days, dayNames[d])
			}
		}
		if len(days) > 0 {
			base = base + ":" + strings.Join(days, ",")
		}
	}

	if r.Mode == models.RecurModeFromCompletion {
		return base + "~"
	}
	return base
}
