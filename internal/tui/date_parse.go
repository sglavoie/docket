package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// ParseDate parses a natural language date string into a time.Time.
// This is a convenience wrapper around util.ParseCalendarDate, so every
// day-granular surface refuses the defer-only hour form by name.
func ParseDate(input string) (time.Time, error) {
	return util.ParseCalendarDate(input)
}
