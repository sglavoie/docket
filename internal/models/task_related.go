package models

import (
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// AddRelatedTo adds a related task ID if not already present.
// Returns an error if relatedID is the task's own ID (self-reference).
func (t *Task) AddRelatedTo(relatedID string) error {
	if relatedID == t.ID {
		return fmt.Errorf("task cannot be related to itself")
	}
	var added bool
	t.RelatedTo, added = util.SliceAdd(t.RelatedTo, relatedID)
	if added {
		t.UpdatedAt = time.Now()
	}
	return nil
}

// RemoveRelatedTo removes a related task ID if present.
func (t *Task) RemoveRelatedTo(relatedID string) {
	var removed bool
	t.RelatedTo, removed = util.SliceRemove(t.RelatedTo, relatedID)
	if removed {
		t.UpdatedAt = time.Now()
	}
}

// HasRelatedTo returns true if the task is related to the given task ID.
func (t *Task) HasRelatedTo(relatedID string) bool {
	return slices.Contains(t.RelatedTo, relatedID)
}
