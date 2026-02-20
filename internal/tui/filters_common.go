package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// CommonFilterState contains the filter fields shared across views that support
// the F-prefix filter system. Views embed this struct directly.
type CommonFilterState struct {
	Type     models.TaskType   // filter by type (epic/story/task/bug/idea)
	Status   models.TaskStatus // filter by status
	Priority int               // 0=all, 1=high only (<=2)
	Tags     map[string]bool   // filter by tag names (AND logic)
	DueDate  DueDateFilter     // filter by due date
	Attrs    map[string]string // type.attribute -> value
	ScoreMin float64           // minimum score threshold (0 = no filter)
}

// passesCommonFilters checks if a task passes the shared filter criteria.
// Views call this from their own passesFilters method before applying
// view-specific logic. The projectID and cfg are needed for score filtering.
func passesCommonFilters(task *models.Task, f *CommonFilterState, projectID string, cfg *config.Config) bool {
	if f.Type != "" && task.Type != f.Type {
		return false
	}
	if f.Status != "" && task.Status != f.Status {
		return false
	}
	if f.Priority == 1 && task.Priority > 2 {
		return false
	}
	if len(f.Tags) > 0 {
		for tag := range f.Tags {
			if !task.HasTag(tag) {
				return false
			}
		}
	}
	if f.DueDate != DueDateAll {
		switch f.DueDate {
		case DueDateOverdue:
			if !task.IsOverdue() {
				return false
			}
		case DueDateToday:
			if !task.IsDueToday() {
				return false
			}
		case DueDateThisWeek:
			if !task.IsDueThisWeek() {
				return false
			}
		case DueDateNone:
			if task.DueDate != nil {
				return false
			}
		}
	}
	if !passesAttrFilters(task, f.Attrs) {
		return false
	}
	if f.ScoreMin > 0 && cfg != nil {
		if profile := cfg.GetProjectScoringProfile(projectID); profile != nil {
			if scoring.NormalizedScore(task, profile) < f.ScoreMin {
				return false
			}
		}
	}
	return true
}

// hasActiveCommonFilters returns true if any common filter is non-default.
func hasActiveCommonFilters(f *CommonFilterState) bool {
	return f.Type != "" ||
		f.Status != "" ||
		f.Priority != 0 ||
		len(f.Tags) > 0 ||
		f.DueDate != DueDateAll ||
		len(f.Attrs) > 0 ||
		f.ScoreMin > 0
}

// clearCommonFilters resets all common filters to defaults.
func clearCommonFilters(f *CommonFilterState) {
	f.Type = ""
	f.Status = ""
	f.Priority = 0
	f.Tags = make(map[string]bool)
	f.DueDate = DueDateAll
	f.Attrs = make(map[string]string)
	f.ScoreMin = 0
}

// commonFilterParts returns the list of active filter description strings.
func commonFilterParts(f *CommonFilterState) []string {
	var parts []string

	if f.Priority == 1 {
		parts = append(parts, "!Hi")
	}
	if f.Type != "" {
		parts = append(parts, "type:"+string(f.Type))
	}
	if f.Status != "" {
		parts = append(parts, "status:"+string(f.Status))
	}
	if len(f.Tags) > 0 {
		var tagNames []string
		for tag := range f.Tags {
			tagNames = append(tagNames, "#"+tag)
		}
		parts = append(parts, strings.Join(tagNames, " "))
	}
	switch f.DueDate {
	case DueDateOverdue:
		parts = append(parts, "due:overdue")
	case DueDateToday:
		parts = append(parts, "due:today")
	case DueDateThisWeek:
		parts = append(parts, "due:week")
	case DueDateNone:
		parts = append(parts, "due:none")
	}
	if len(f.Attrs) > 0 {
		var attrParts []string
		for k, v := range f.Attrs {
			attrParts = append(attrParts, k+"="+v)
		}
		parts = append(parts, "attr:"+strings.Join(attrParts, ","))
	}
	if f.ScoreMin > 0 {
		parts = append(parts, fmt.Sprintf("score>=%.0f", f.ScoreMin))
	}

	return parts
}

// renderCommonFilterIndicator returns a compact badge of all active common filters.
// Returns empty string if no filters are active.
func renderCommonFilterIndicator(f *CommonFilterState) string {
	return renderFilterBadge(commonFilterParts(f))
}

// buildTagListFromTasks scans tasks and builds a sorted list of tags with counts.
func buildTagListFromTasks(tasks []*models.Task) []TagInfo {
	tagCounts := make(map[string]int)
	for _, task := range tasks {
		for _, tag := range task.Tags {
			tagCounts[tag]++
		}
	}

	tags := make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		tags = append(tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}

// renderFilterBadge renders a styled filter badge from a parts list.
// Returns empty string if parts is empty.
func renderFilterBadge(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return FilterBadgeStyle.Render("Filters: " + strings.Join(parts, " "))
}
