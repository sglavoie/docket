package models

import (
	"fmt"
	"strings"
	"time"
)

// NormalizeTaskContext returns the canonical spelling for one task context.
// Context values intentionally share the lowercase ASCII kebab-case contract
// with tags while remaining a distinct, single-valued task field.
func NormalizeTaskContext(raw string) (string, error) {
	context := NormalizeTag(raw)
	if err := ValidateTag(context); err != nil {
		return "", contextValidationError(err)
	}
	return context, nil
}

// ValidateTaskContext checks a stored task context. The empty string means the
// optional field is unset; non-empty values share the durable tag contract.
func ValidateTaskContext(value string) error {
	if value == "" {
		return nil
	}
	if err := ValidateTag(value); err != nil {
		return contextValidationError(err)
	}
	return nil
}

func contextValidationError(err error) error {
	return fmt.Errorf("%s", strings.Replace(err.Error(), "tag", "context", 1))
}

// HasContext reports whether the task has the requested context. Both the
// requested and stored values are normalized before comparison.
func (t *Task) HasContext(value string) bool {
	normalized, err := NormalizeTaskContext(value)
	if err != nil || normalized == "" {
		return false
	}
	stored, err := NormalizeTaskContext(t.Context)
	return err == nil && stored == normalized
}

// SetContext stores a valid normalized context and bumps UpdatedAt only when
// the durable value changes. Invalid values are ignored, matching AddTag.
func (t *Task) SetContext(value string) {
	normalized, err := NormalizeTaskContext(value)
	if err != nil || t.Context == normalized {
		return
	}
	t.Context = normalized
	t.UpdatedAt = time.Now()
}

// ClearContext unsets the task context and bumps UpdatedAt only on change.
func (t *Task) ClearContext() {
	if t.Context == "" {
		return
	}
	t.Context = ""
	t.UpdatedAt = time.Now()
}
