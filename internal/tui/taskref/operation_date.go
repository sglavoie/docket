package taskref

import "time"

// CloneOperationDate copies a date an operation carries, so a picker preset and
// the task it was read from never share one pointer.
func CloneOperationDate(date *time.Time) *time.Time {
	if date == nil {
		return nil
	}
	cloned := *date
	return &cloned
}
