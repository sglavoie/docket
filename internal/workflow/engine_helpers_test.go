package workflow

import (
	"time"
)

// Helper function for creating time pointers in tests
func timePtr(t time.Time) *time.Time {
	return &t
}
