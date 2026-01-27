package models

import (
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// AddRelatedTo adds a related task ID if not already present.
func (t *Task) AddRelatedTo(relatedID string) {
	if util.SliceContains(t.RelatedTo, relatedID) {
		return
	}
	t.RelatedTo = append(t.RelatedTo, relatedID)
	t.UpdatedAt = time.Now()
}

// RemoveRelatedTo removes a related task ID if present.
func (t *Task) RemoveRelatedTo(relatedID string) {
	if newRelated, removed := util.SliceRemove(t.RelatedTo, relatedID); removed {
		t.RelatedTo = newRelated
		t.UpdatedAt = time.Now()
	}
}

// HasRelatedTo returns true if the task is related to the given task ID.
func (t *Task) HasRelatedTo(relatedID string) bool {
	return util.SliceContains(t.RelatedTo, relatedID)
}

// ClearRelatedTo removes all related task references.
func (t *Task) ClearRelatedTo() {
	if len(t.RelatedTo) > 0 {
		t.RelatedTo = []string{}
		t.UpdatedAt = time.Now()
	}
}
