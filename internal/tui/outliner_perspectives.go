package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

// SectionOrder is the canonical GTD display order used by the All perspective
// in the cross-project outliner.
var SectionOrder = []string{
	models.SectionInbox,
	"",
	models.SectionNext,
	models.SectionWaiting,
	models.SectionReview,
	models.SectionStandby,
	models.SectionSomeday,
}

// BuildSectionOrder extends SectionOrder with custom sections found in the
// given tasks. Custom sections are appended after canonical GTD sections in
// alphabetical order.
func BuildSectionOrder(tasks []*models.Task) []string {
	customSet := make(map[string]bool)
	for _, t := range tasks {
		if t.CustomSection != "" {
			customSet[t.CustomSection] = true
		}
	}
	if len(customSet) == 0 {
		return SectionOrder
	}
	customs := make([]string, 0, len(customSet))
	for name := range customSet {
		customs = append(customs, name)
	}
	sort.Strings(customs)
	result := make([]string, len(SectionOrder), len(SectionOrder)+len(customs))
	copy(result, SectionOrder)
	return append(result, customs...)
}

// BuildCrossProjectSectionOrder extends SectionOrder with custom sections
// found across all project groups. Custom sections are appended after
// canonical GTD sections in alphabetical order.
func BuildCrossProjectSectionOrder(groups []projectTasksGroup) []string {
	customSet := make(map[string]bool)
	for _, g := range groups {
		for _, t := range g.tasks {
			if t.CustomSection != "" {
				customSet[t.CustomSection] = true
			}
		}
	}
	if len(customSet) == 0 {
		return SectionOrder
	}
	customs := make([]string, 0, len(customSet))
	for name := range customSet {
		customs = append(customs, name)
	}
	sort.Strings(customs)
	result := make([]string, len(SectionOrder), len(SectionOrder)+len(customs))
	copy(result, SectionOrder)
	return append(result, customs...)
}

// Perspective defines a named filter+sort preset for the outliner view.
// The Filter predicate returns true for tasks that should be visible.
// A nil Filter means all tasks are shown (no filtering).
type Perspective struct {
	Name   string
	Key    int // 0-9 numeric key binding
	Filter func(t *models.Task) bool
}

// Perspective key constants for commonly referenced perspectives.
const (
	PerspectiveSnoozed   = 0
	PerspectiveAll       = 1
	PerspectiveCompleted = 8
)

// perspectives holds the ten hardcoded perspective presets, indexed 0-9.
// Access via PerspectivePresets().
var perspectives = [10]Perspective{
	{
		Name: "Snoozed",
		Key:  0,
		Filter: func(t *models.Task) bool {
			return t.IsSnoozed()
		},
	},
	{
		Name: "All",
		Key:  1,
		Filter: func(t *models.Task) bool {
			return !t.Status.IsComplete() && !t.IsSnoozed()
		},
	},
	{
		Name: "Active",
		Key:  2,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusInProgress && !t.IsSnoozed()
		},
	},
	{
		Name: "Next",
		Key:  3,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusTodo && t.Priority >= 1 && t.Priority <= 2 && !t.IsSnoozed()
		},
	},
	{
		Name: "Inbox",
		Key:  4,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusTodo && len(t.Tags) == 0 && !t.IsSnoozed()
		},
	},
	{
		Name: "Standby",
		Key:  5,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusStandby
		},
	},
	{
		Name: "Waiting",
		Key:  6,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusBlocked && !t.IsSnoozed()
		},
	},
	{
		Name: "Someday",
		Key:  7,
		Filter: func(t *models.Task) bool {
			return t.Status == models.StatusTodo && t.Priority >= 4 && t.Priority <= 5 && !t.IsSnoozed()
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
			return t.Status == models.StatusReview && !t.IsSnoozed()
		},
	},
}

// PerspectivePresets returns the ten hardcoded perspective presets.
// The returned slice is indexed 0-9 corresponding to keys 0-9.
func PerspectivePresets() [10]Perspective {
	return perspectives
}

// PerspectiveByKey returns the perspective for the given key (0-9) and true,
// or an empty Perspective and false if the key is out of range.
func PerspectiveByKey(key int) (Perspective, bool) {
	if key < 0 || key > 9 {
		return Perspective{}, false
	}
	return perspectives[key], true
}
