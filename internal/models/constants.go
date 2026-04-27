package models

import "fmt"

// Priority constants define the valid range and default value for task priorities.
const (
	PriorityMin     = 1 // Critical/Urgent
	PriorityMax     = 5 // Minimal
	PriorityDefault = 3 // Medium
)

// ValidatePriority checks that a priority value is in the valid range.
// If allowZero is true, 0 is accepted (meaning "use default").
func ValidatePriority(value int, allowZero bool) error {
	if allowZero && value == 0 {
		return nil
	}
	if value < PriorityMin || value > PriorityMax {
		return fmt.Errorf("priority must be between %d and %d", PriorityMin, PriorityMax)
	}
	return nil
}
