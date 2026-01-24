package models

import (
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// AddTag adds a tag if not already present.
func (t *Task) AddTag(tag string) {
	if util.SliceContains(t.Tags, tag) {
		return
	}
	t.Tags = append(t.Tags, tag)
	t.UpdatedAt = time.Now()
}

// RemoveTag removes a tag if present.
func (t *Task) RemoveTag(tag string) {
	if newTags, removed := util.SliceRemove(t.Tags, tag); removed {
		t.Tags = newTags
		t.UpdatedAt = time.Now()
	}
}

// HasTag returns true if the task has the given tag.
func (t *Task) HasTag(tag string) bool {
	return util.SliceContains(t.Tags, tag)
}
