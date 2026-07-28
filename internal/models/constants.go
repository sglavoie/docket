package models

import "fmt"

// Priority constants define the valid range and default value for task priorities.
//
// PriorityUnset is the one canonical "no priority assigned" value. It is a
// durable state, not a placeholder: it persists as `priority: 0`, parses back as
// zero, renders no badge, and never counts as high priority. Every path that
// needs a number instead of a presence answer resolves it through
// EffectivePriority, so an unset task neither outranks critical work nor sorts
// below the scale.
const (
	PriorityUnset   = 0 // Unset — no priority assigned
	PriorityMin     = 1 // Critical/Urgent
	PriorityMax     = 5 // Minimal
	PriorityDefault = 3 // Medium
)

// ValidatePriority checks that a priority value is in the valid range.
// If allowZero is true, PriorityUnset is accepted.
func ValidatePriority(value int, allowZero bool) error {
	if allowZero && value == PriorityUnset {
		return nil
	}
	if value < PriorityMin || value > PriorityMax {
		return fmt.Errorf("priority must be between %d and %d", PriorityMin, PriorityMax)
	}
	return nil
}

// EffectivePriority resolves a stored priority to the number that ordering,
// filtering, and scoring should use. PriorityUnset — and any value outside the
// scale, which only legacy or hand-edited input can produce — reads as
// PriorityDefault.
func EffectivePriority(value int) int {
	if ValidatePriority(value, false) != nil {
		return PriorityDefault
	}
	return value
}

// IsHighPriority reports whether a priority belongs to the two most urgent
// buckets, which is what conditional display and high-priority grouping key off.
// An unset priority is not high priority.
func IsHighPriority(value int) bool {
	return EffectivePriority(value) <= 2
}
