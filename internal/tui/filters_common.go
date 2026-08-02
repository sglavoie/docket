package tui

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// CommonFilterState contains filter fields shared across views.
type CommonFilterState struct {
	States      models.WorkflowStateSet // filter by one or more canonical workflow states
	Priority    int                     // 0=all, N>0 keeps tasks whose effective priority is <= N
	Tags        map[string]bool         // filter by tag names (AND logic)
	DueDate     DueDateFilter           // filter by due date
	Actionable  bool                    // only show tasks ready to work on
	GitHubIssue bool                    // only show tasks linked to a GitHub issue
	Blocked     bool                    // only show tasks blocked by an unresolved dependency
	Snoozed     bool                    // only show currently snoozed tasks
	Pinned      bool                    // only show pinned tasks

	// statesSet distinguishes no state predicate from an explicit empty
	// match-nothing set produced by intersecting contradictory legacy filters.
	statesSet bool
}

// copyCommonFilterState returns a deep copy of a CommonFilterState.
func copyCommonFilterState(src *CommonFilterState) *CommonFilterState {
	dst := &CommonFilterState{
		States:      cloneCommonWorkflowStateSet(src.States),
		Priority:    src.Priority,
		DueDate:     src.DueDate,
		Actionable:  src.Actionable,
		GitHubIssue: src.GitHubIssue,
		Blocked:     src.Blocked,
		Snoozed:     src.Snoozed,
		Pinned:      src.Pinned,
		statesSet:   src.statesSet,
	}
	if len(src.Tags) > 0 {
		dst.Tags = make(map[string]bool, len(src.Tags))
		maps.Copy(dst.Tags, src.Tags)
	}
	return dst
}

func cloneCommonWorkflowStateSet(states models.WorkflowStateSet) models.WorkflowStateSet {
	cloned, err := models.NewWorkflowStateSet(states.Values()...)
	if err != nil {
		panic(err)
	}
	return cloned
}

func (f *CommonFilterState) hasStateFilter() bool {
	return f.statesSet || !f.States.IsEmpty()
}

func (f *CommonFilterState) setStateFilter(states models.WorkflowStateSet, active bool) {
	f.States = cloneCommonWorkflowStateSet(states)
	f.statesSet = active || !states.IsEmpty()
}

// passesCommonFilters checks if a task passes the shared filter criteria.
// Views call this from their own passesFilters method before applying
// view-specific logic. The project is needed for the derived
// dependency-blocked filter, which resolves unresolved dependencies within the
// project. A nil project disables that project-scoped filter; the state filter is
// project-independent because WorkflowState is derived from the task alone.
// passesCommonFiltersWithResolverAt evaluates the same shared facets while
// resolving dependency blocking against one workspace snapshot. Callers that
// do not have a workspace resolver retain the legacy project-local behavior;
// the canonical Outliner always supplies one for its complete projection.
func passesCommonFiltersWithResolverAt(
	task *models.Task,
	f *CommonFilterState,
	project *models.Project,
	resolver *workspaceFilterResolver,
	now time.Time,
) bool {
	if f.hasStateFilter() && !f.States.Contains(task.State()) {
		return false
	}
	if f.Blocked {
		return false
	}
	if f.Priority > 0 && models.EffectivePriority(task.Priority) > f.Priority {
		return false
	}
	if f.GitHubIssue && !task.HasGitHubIssue() {
		return false
	}
	if f.Snoozed && !task.IsSnoozedAt(now) {
		return false
	}
	if f.Pinned && !task.Pinned {
		return false
	}
	if len(f.Tags) > 0 {
		for tag := range f.Tags {
			if !task.HasTag(tag) {
				return false
			}
		}
	}
	if !passesDueDateFilterAt(task, f.DueDate, now) {
		return false
	}
	return true
}

func passesDueDateFilterAt(task *models.Task, filter DueDateFilter, now time.Time) bool {
	switch filter {
	case DueDateOverdue:
		return task.DueDate != nil &&
			!task.IsTerminalState() &&
			util.DaysBetween(now, *task.DueDate) < 0
	case DueDateToday:
		return task.DueDate != nil && util.DaysBetween(now, *task.DueDate) == 0
	case DueDateThisWeek:
		if task.DueDate == nil {
			return false
		}
		days := util.DaysBetween(now, *task.DueDate)
		return days >= 0 && days < 7
	case DueDateNone:
		return task.DueDate == nil
	default:
		return true
	}
}

// hasActiveCommonFilters returns true if any common filter is non-default.
func hasActiveCommonFilters(f *CommonFilterState) bool {
	return f.hasStateFilter() ||
		f.Priority != 0 ||
		len(f.Tags) > 0 ||
		f.DueDate != DueDateAll ||
		f.Actionable ||
		f.GitHubIssue ||
		f.Blocked ||
		f.Snoozed ||
		f.Pinned
}

// clearCommonFilters resets all common filters to defaults.
func clearCommonFilters(f *CommonFilterState) {
	f.States = models.WorkflowStateSet{}
	f.statesSet = false
	f.Priority = 0
	f.Tags = make(map[string]bool)
	f.DueDate = DueDateAll
	f.Actionable = false
	f.GitHubIssue = false
	f.Blocked = false
	f.Snoozed = false
	f.Pinned = false
}

// commonFilterParts returns the list of active filter description strings.
func commonFilterParts(f *CommonFilterState) []string {
	var parts []string

	if f.Priority > 0 {
		parts = append(parts, fmt.Sprintf("!<=%d", f.Priority))
	}
	if f.hasStateFilter() {
		states := f.States.Values()
		stateNames := make([]string, len(states))
		for i, state := range states {
			stateNames[i] = string(state)
		}
		value := "none"
		if len(stateNames) > 0 {
			value = strings.Join(stateNames, ",")
		}
		parts = append(parts, "state:"+value)
	}
	if len(f.Tags) > 0 {
		tagNames := make([]string, 0, len(f.Tags))
		for tag := range f.Tags {
			tagNames = append(tagNames, tag)
		}
		sort.Strings(tagNames)
		for i, tag := range tagNames {
			tagNames[i] = "#" + tag
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
	default:
	}
	if f.Actionable {
		parts = append(parts, "actionable")
	}
	if f.GitHubIssue {
		parts = append(parts, "issue")
	}
	if f.Snoozed {
		parts = append(parts, "snoozed")
	}
	if f.Pinned {
		parts = append(parts, "pinned")
	}
	if f.Blocked {
		parts = append(parts, "blocked")
	}

	return parts
}

// CollectTagNamesForProject returns sorted unique tag names from the project's
// catalog and task usage.
func CollectTagNamesForProject(project *models.Project) []string {
	return tagNamesFromInfos(BuildProjectTagList(project, false))
}

func tagNamesFromInfos(infos []TagInfo) []string {
	tags := make([]string, len(infos))
	for i, info := range infos {
		tags[i] = info.Name
	}
	return tags
}

// FilterState contains all task filtering options including view-specific fields
// beyond the common set. Used by ProjectTree and Outliner.
type FilterState struct {
	CommonFilterState // embedded common filter fields
}

// NewFilterState returns an initialized FilterState with allocated maps.
func NewFilterState() FilterState {
	return FilterState{
		CommonFilterState: newFilterState(),
	}
}

// renderFilterBadge renders a styled filter badge from a parts list.
// Returns empty string if parts is empty.
func renderFilterBadge(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return FilterBadgeStyle.Render("Filters: " + strings.Join(parts, " "))
}

// newFilterState returns an initialized CommonFilterState with allocated maps.
func newFilterState() CommonFilterState {
	return CommonFilterState{
		Tags: make(map[string]bool),
	}
}
