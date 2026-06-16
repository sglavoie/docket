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

// BuildSectionOrder returns the canonical GTD section-header order. Custom
// sections are surfaced as tabs, not as collapsible section headers.
func BuildSectionOrder(tasks []*models.Task) []string {
	return sectionOrderCopy()
}

// BuildCrossProjectSectionOrder returns the canonical GTD section-header order
// for aggregate rows. Cross-project custom sections are surfaced as tabs, not
// as collapsible section headers.
func BuildCrossProjectSectionOrder(groups []projectTasksGroup) []string {
	return sectionOrderCopy()
}

// headerSectionForTask returns the GTD section used for collapsible section
// headers. Custom-section tasks are grouped under Active (the empty section);
// custom membership remains available through Task.InSection and tab filters.
func headerSectionForTask(t *models.Task) string {
	if t == nil || t.CustomSection != "" {
		return ""
	}
	return t.Section
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
