package models

import "fmt"

// Priority constants define the valid range and default value for task priorities.
//
// PriorityUnset is the one canonical "no priority assigned" value. It is a
// durable state, not a placeholder: it persists as `priority: 0`, parses back as
// zero, and is the only priority that renders no badge. Every path that needs a
// number instead of a presence answer resolves it through EffectivePriority, so
// an unset task neither outranks critical work nor sorts below the scale.
const (
	PriorityUnset  = 0 // Unset — no priority assigned
	PriorityHigh   = 1 // High
	PriorityMedium = 2 // Medium
	PriorityLow    = 3 // Low

	// Compile-time aliases retained while older callers are migrated.
	PriorityMin = PriorityHigh
	PriorityMax = PriorityLow
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

// EffectivePriority returns the ascending sort rank for a stored priority.
// Unset and invalid values sort after Low.
func EffectivePriority(value int) int {
	if value == PriorityUnset || ValidatePriority(value, false) != nil {
		return PriorityLow + 1
	}
	return value
}

// PriorityName returns the canonical display vocabulary for a stored priority.
func PriorityName(value int) string {
	switch value {
	case PriorityUnset:
		return "Unset"
	case PriorityHigh:
		return "High"
	case PriorityMedium:
		return "Medium"
	case PriorityLow:
		return "Low"
	default:
		return ""
	}
}
