package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// Perspective defines a named filter+sort preset for the outliner view.
// The Filter predicate returns true for tasks that should be visible.
// A nil Filter means all tasks are shown (no filtering).
type Perspective struct {
	Name   string
	Key    int // 1-9 numeric key binding
	Filter func(t *models.Task) bool
}

// perspectives holds the nine hardcoded perspective presets, indexed 1-9.
// Access via PerspectivePresets().
var perspectives = [9]Perspective{
	{
		Name: "All",
		Key:  1,
		// nil filter — show everything
	},
	{
		Name: "Active",
		Key:  2,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusInProgress
		},
	},
	{
		Name: "Next",
		Key:  3,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusTodo && t.Priority >= 1 && t.Priority <= 2
		},
	},
	{
		Name: "Inbox",
		Key:  4,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusTodo && len(t.Tags) == 0
		},
	},
	{
		Name: "Urgent",
		Key:  5,
		Filter: func(t *models.Task) bool {
			if t.Status != models.StatusInProgress && t.Status != models.StatusTodo {
				return false
			}
			if t.Priority == 1 {
				return true
			}
			if t.DueDate != nil && time.Until(*t.DueDate) <= 3*24*time.Hour {
				return true
			}
			return false
		},
	},
	{
		Name: "Waiting",
		Key:  6,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusBlocked
		},
	},
	{
		Name: "Someday",
		Key:  7,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusTodo && t.Priority >= 4 && t.Priority <= 5
		},
	},
	{
		Name: "Completed",
		Key:  8,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusDone
		},
	},
	{
		Name: "Review",
		Key:  9,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusReview
		},
	},
}

// PerspectivePresets returns the nine hardcoded perspective presets.
// The returned slice is indexed 0-8 corresponding to keys 1-9.
func PerspectivePresets() [9]Perspective {
	return perspectives
}

// PerspectiveByKey returns the perspective for the given key (1-9) and true,
// or an empty Perspective and false if the key is out of range.
func PerspectiveByKey(key int) (Perspective, bool) {
	if key < 1 || key > 9 {
		return Perspective{}, false
	}
	return perspectives[key-1], true
}
