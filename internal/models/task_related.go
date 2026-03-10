package models

import (
	"fmt"
	"slices"
	"time"
)

// AddRelatedTo adds a related task ID if not already present.
// Returns an error if relatedID is the task's own ID (self-reference).
func (t *Task) AddRelatedTo(relatedID string) error {
	if relatedID == t.ID {
		return fmt.Errorf("task cannot be related to itself")
	}
	if slices.Contains(t.RelatedTo, relatedID) {
		return nil
	}
	t.RelatedTo = append(t.RelatedTo, relatedID)
	t.UpdatedAt = time.Now()
	return nil
}

// RemoveRelatedTo removes a related task ID if present.
func (t *Task) RemoveRelatedTo(relatedID string) {
	if i := slices.Index(t.RelatedTo, relatedID); i >= 0 {
		t.RelatedTo = slices.Delete(t.RelatedTo, i, i+1)
		t.UpdatedAt = time.Now()
	}
}

// HasRelatedTo returns true if the task is related to the given task ID.
func (t *Task) HasRelatedTo(relatedID string) bool {
	return slices.Contains(t.RelatedTo, relatedID)
}

// ClearRelatedTo removes all related task references.
func (t *Task) ClearRelatedTo() {
	if len(t.RelatedTo) > 0 {
		t.RelatedTo = []string{}
		t.UpdatedAt = time.Now()
	}
}
