package capturetext

import (
	"time"

	"github.com/sglavoie/docket/internal/util"
)

// ParseDateAt parses a natural language date string into a time.Time,
// resolving relative days against now. This is a convenience wrapper around
// util.ParseCalendarDateAt, so every day-granular surface refuses the
// defer-only hour form by name.
func ParseDateAt(input string, now time.Time) (time.Time, error) {
	return util.ParseCalendarDateAt(input, now)
}

// ParseDueDateValue parses a due-date value that may carry an optional trailing
// hh:mm clock time. The returned time's clock is the due time (local midnight
// when no time is given); models.Task.DueHasTime derives from that clock, so
// callers only need the time.Time. Like ParseDate it refuses the hour-offset
// form (+Nh): hours are defer-only.
func ParseDueDateValue(input string) (time.Time, error) {
	due, _, err := util.ParseDueDate(input)
	return due, err
}
