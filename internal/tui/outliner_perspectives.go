package tui

import "github.com/sglavoie/tt/internal/models"

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

// BuildSectionOrder returns the canonical GTD section-header order. Pinned
// tags are surfaced as tabs, not as collapsible section headers.
func BuildSectionOrder(tasks []*models.Task) []string {
	return sectionOrderCopy()
}

// BuildCrossProjectSectionOrder returns the canonical GTD section-header order
// for aggregate rows. Cross-project pinned tags are surfaced as tabs, not
// as collapsible section headers.
func BuildCrossProjectSectionOrder(groups []projectTasksGroup) []string {
	return sectionOrderCopy()
}

// headerSectionForTask returns the GTD section used for collapsible section headers.
func headerSectionForTask(t *models.Task) string {
	if t == nil {
		return ""
	}
	return legacySectionForStateGroup(headerStateGroupForTask(t))
}

// headerStateGroupForTask is the authoritative classifier used by All-mode
// grouping. headerSectionForTask above remains the thin visible-surface adapter.
func headerStateGroupForTask(t *models.Task) models.WorkflowStateGroup {
	return legacyAllPresentationGroup(taskStateGroup(t))
}

func sectionOrderCopy() []string {
	result := make([]string, len(SectionOrder))
	copy(result, SectionOrder)
	return result
}

// Perspective defines a named filter+sort preset for the outliner view.
// The Filter predicate returns true for tasks that should be visible.
// A nil Filter means all tasks are shown (no filtering).
type Perspective struct {
	ID     int // stable internal ID persisted in saved filters and view state
	Name   string
	Key    int // 0-9 numeric key binding
	Filter func(t *models.Task) bool
}

// Perspective ID constants. These values are persisted in saved filters and
// view state, so they must remain stable even when shortcut keys are remapped.
const (
	PerspectiveSnoozed = iota
	PerspectiveAll
	PerspectiveActive
	PerspectiveNext
	PerspectiveInbox
	PerspectiveStandby
	PerspectiveWaiting
	PerspectiveSomeday
	PerspectiveCompleted
	PerspectiveReview
)

// perspectives holds the ten hardcoded perspective presets, indexed by their
// stable IDs. Shortcut keys deliberately differ for Completed, Review, and
// Snoozed so the visible cycle ends with Completed without changing persisted
// perspective meanings.
var perspectives = [10]Perspective{
	{
		ID:   PerspectiveSnoozed,
		Name: "Snoozed",
		Key:  9,
		Filter: func(t *models.Task) bool {
			return t.IsSnoozed()
		},
	},
	{
		ID:   PerspectiveAll,
		Name: "All",
		Key:  1,
		Filter: func(t *models.Task) bool {
			return !t.IsTerminalState() && !t.IsSnoozed()
		},
	},
	{
		ID:   PerspectiveActive,
		Name: "Active",
		Key:  2,
		Filter: func(t *models.Task) bool {
			return t.InSection("") && !t.IsSnoozed()
		},
	},
	{
		ID:   PerspectiveNext,
		Name: "Next",
		Key:  3,
		Filter: func(t *models.Task) bool {
			return t.InSection(models.SectionNext) && !t.IsSnoozed()
		},
	},
	{
		ID:   PerspectiveInbox,
		Name: "Inbox",
		Key:  4,
		Filter: func(t *models.Task) bool {
			return t.InSection(models.SectionInbox) && !t.IsSnoozed()
		},
	},
	{
		ID:   PerspectiveStandby,
		Name: "Standby",
		Key:  5,
		Filter: func(t *models.Task) bool {
			return t.InSection(models.SectionStandby) && !t.IsSnoozed()
		},
	},
	{
		ID:   PerspectiveWaiting,
		Name: "Waiting",
		Key:  6,
		Filter: func(t *models.Task) bool {
			return t.InSection(models.SectionWaiting) && !t.IsSnoozed()
		},
	},
	{
		ID:   PerspectiveSomeday,
		Name: "Someday",
		Key:  7,
		Filter: func(t *models.Task) bool {
			return t.InSection(models.SectionSomeday) && !t.IsSnoozed()
		},
	},
	{
		ID:   PerspectiveCompleted,
		Name: "Completed",
		Key:  0,
		Filter: func(t *models.Task) bool {
			return t.InSection(models.SectionCompleted)
		},
	},
	{
		ID:   PerspectiveReview,
		Name: "Review",
		Key:  8,
		Filter: func(t *models.Task) bool {
			return t.InSection(models.SectionReview) && !t.IsSnoozed()
		},
	},
}

// perspectiveCycleOrder is the canonical visible rotation shared by every
// TUI surface that cycles perspectives.
var perspectiveCycleOrder = [...]int{
	PerspectiveAll,
	PerspectiveActive,
	PerspectiveNext,
	PerspectiveInbox,
	PerspectiveStandby,
	PerspectiveWaiting,
	PerspectiveSomeday,
	PerspectiveReview,
	PerspectiveSnoozed,
	PerspectiveCompleted,
}

// PerspectivePresets returns the ten hardcoded perspective presets.
// The returned array is indexed by stable perspective ID, not shortcut key.
func PerspectivePresets() [10]Perspective {
	return perspectives
}

// PerspectiveByID returns the perspective for the given stable ID and true,
// or an empty Perspective and false if the ID is out of range.
func PerspectiveByID(id int) (Perspective, bool) {
	if id < 0 || id >= len(perspectives) {
		return Perspective{}, false
	}
	return perspectives[id], true
}

// PerspectiveByKey returns the perspective bound to the given shortcut key
// (0-9) and true, or an empty Perspective and false if the key is out of range.
func PerspectiveByKey(key int) (Perspective, bool) {
	if key < 0 || key > 9 {
		return Perspective{}, false
	}
	for _, perspective := range perspectives {
		if perspective.Key == key {
			return perspective, true
		}
	}
	return Perspective{}, false
}

// CyclePerspective advances a stable perspective ID by delta in the canonical
// visible rotation. Invalid IDs enter at All when moving forward and Completed
// when moving backward.
func CyclePerspective(current, delta int) int {
	for idx, id := range perspectiveCycleOrder {
		if id != current {
			continue
		}
		next := (idx + delta) % len(perspectiveCycleOrder)
		if next < 0 {
			next += len(perspectiveCycleOrder)
		}
		return perspectiveCycleOrder[next]
	}
	if delta < 0 {
		return PerspectiveCompleted
	}
	return PerspectiveAll
}
