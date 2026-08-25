package tui

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// CommonFilterState contains filter fields shared across views.
type CommonFilterState struct {
	States        models.WorkflowStateSet // filter by one or more canonical workflow states
	Priority      int                     // 0=all, N>0 keeps tasks whose effective priority is <= N
	Tags          map[string]bool         // filter by tag names (AND logic)
	DueDate       DueDateFilter           // filter by due date
	CompletionAge CompletionAgeFilter     // local-calendar age threshold for completed tasks
	Hierarchy     HierarchyFilter         // filter by hierarchy position
	Stale         bool                    // only show open work untouched for at least 90 local days
	Deferred      bool                    // only show currently deferred tasks
	Pinned        bool                    // only show pinned tasks

	// statesSet distinguishes no state predicate from an explicit empty
	// match-nothing set produced by intersecting contradictory legacy filters.
	statesSet bool
	// completionAgeSet distinguishes an inactive cleanup facet from the active
	// "all ages" preset, which still excludes invalid CompletedAt values.
	completionAgeSet bool
}

// copyCommonFilterState returns a deep copy of a CommonFilterState.
func copyCommonFilterState(src *CommonFilterState) *CommonFilterState {
	dst := &CommonFilterState{
		States:           cloneCommonWorkflowStateSet(src.States),
		Priority:         src.Priority,
		DueDate:          src.DueDate,
		CompletionAge:    src.CompletionAge,
		Hierarchy:        src.Hierarchy,
		Stale:            src.Stale,
		Deferred:         src.Deferred,
		Pinned:           src.Pinned,
		statesSet:        src.statesSet,
		completionAgeSet: src.completionAgeSet,
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

func (f *CommonFilterState) hasCompletionAgeFilter() bool {
	return f.completionAgeSet
}

func (f *CommonFilterState) setCompletionAgeFilter(filter CompletionAgeFilter) {
	f.CompletionAge = filter
	f.completionAgeSet = true
}

// passesCommonFiltersAt checks whether a task passes the shared filter facets
// at one fixed instant. Workflow-state filtering is explicit and derives only
// from the task's canonical lifecycle state.
//
// The hierarchy facet is deliberately absent: it needs the scope's
// open-direct-child census, and the task arriving here can be a promotion-pass
// clone whose Children were rewritten. Evaluating it here would make "parent"
// mean one thing in All and another in a flat mode. It lives in
// passesHierarchyFilter instead.
func passesCommonFiltersAt(
	task *models.Task,
	f *CommonFilterState,
	now time.Time,
) bool {
	if f.hasStateFilter() && !f.States.Contains(task.State()) {
		return false
	}
	if f.hasCompletionAgeFilter() && !passesCompletionAgeFilterAt(task, f.CompletionAge, now) {
		return false
	}
	if f.Priority > 0 && models.EffectivePriority(task.Priority) > f.Priority {
		return false
	}
	if f.Stale && !taskIsStale(task, now) {
		return false
	}
	if f.Deferred && !task.IsDeferredAt(now) {
		return false
	}
	if f.Pinned && !task.Pinned {
		return false
	}
	if len(f.Tags) > 0 {
		for tag := range f.Tags {
			if !taskMatchesTagFacet(task, tag) {
				return false
			}
		}
	}
	if !passesDueDateFilterAt(task, f.DueDate, now) {
		return false
	}
	return true
}

// taskMatchesTagFacet keeps the task model's tag and context axes distinct
// while letting the existing filter facet match either vocabulary.
func taskMatchesTagFacet(task *models.Task, value string) bool {
	return task != nil && (task.HasTag(value) || task.HasContext(value))
}

func passesCompletionAgeFilterAt(task *models.Task, filter CompletionAgeFilter, now time.Time) bool {
	if task.CompletedAt == nil || task.CompletedAt.IsZero() {
		return false
	}
	threshold := filter.days()
	if threshold == 0 {
		return true
	}
	completedLocal := task.CompletedAt.In(now.Location())
	return util.DaysBetween(completedLocal, now) >= threshold
}

func passesDueDateFilterAt(task *models.Task, filter DueDateFilter, now time.Time) bool {
	switch filter {
	case DueDateOverdue:
		if task.DueDate == nil || task.IsTerminalState() {
			return false
		}
		if task.DueHasTime() {
			return task.DueDate.Before(now)
		}
		return util.DaysBetween(now, *task.DueDate) < 0
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
		f.hasCompletionAgeFilter() ||
		f.Priority != 0 ||
		len(f.Tags) > 0 ||
		f.DueDate != DueDateAll ||
		f.Hierarchy != HierarchyAll ||
		f.Stale ||
		f.Deferred ||
		f.Pinned
}

// commonFilterFacetCount reports independently clearable task facets. The
// terminal-state predicate installed by a completion-age choice is part of
// that one cleanup facet, not a second state facet in the clear count.
func commonFilterFacetCount(f *CommonFilterState) int {
	if f == nil {
		return 0
	}
	count := 0
	if f.hasStateFilter() && !completionAgeOwnsTerminalStates(f) {
		count++
	}
	if f.hasCompletionAgeFilter() {
		count++
	}
	if f.Priority != 0 {
		count++
	}
	if len(f.Tags) > 0 {
		count++
	}
	if f.DueDate != DueDateAll {
		count++
	}
	if f.Hierarchy != HierarchyAll {
		count++
	}
	if f.Stale {
		count++
	}
	if f.Deferred {
		count++
	}
	if f.Pinned {
		count++
	}
	return count
}

func completionAgeOwnsTerminalStates(f *CommonFilterState) bool {
	if f == nil || !f.hasCompletionAgeFilter() || !f.hasStateFilter() {
		return false
	}
	terminal, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	return err == nil && f.States.Equal(terminal)
}

// clearCommonFilters resets all common filters to defaults.
func clearCommonFilters(f *CommonFilterState) {
	f.States = models.WorkflowStateSet{}
	f.statesSet = false
	f.Priority = 0
	f.Tags = make(map[string]bool)
	f.DueDate = DueDateAll
	f.CompletionAge = CompletionAgeAll
	f.completionAgeSet = false
	f.Hierarchy = HierarchyAll
	f.Stale = false
	f.Deferred = false
	f.Pinned = false
}

// commonFilterParts returns the list of active filter description strings.
func commonFilterParts(f *CommonFilterState) []string {
	var parts []string

	if f.Priority > 0 {
		parts = append(parts, "priority:"+models.PriorityName(f.Priority)+"+")
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
	if f.hasCompletionAgeFilter() {
		if days := f.CompletionAge.days(); days > 0 {
			parts = append(parts, fmt.Sprintf("completed:%dd+", days))
		} else {
			parts = append(parts, "completed:all")
		}
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
	if f.Hierarchy != HierarchyAll {
		parts = append(parts, "hierarchy:"+hierarchyFilterLabel(f.Hierarchy))
	}
	if f.Stale {
		parts = append(parts, "stale")
	}
	if f.Deferred {
		parts = append(parts, "deferred")
	}
	if f.Pinned {
		parts = append(parts, "pinned")
	}
	return parts
}

// CollectTagNamesForProject returns sorted unique tag names from the project's
// catalog and task usage.
func CollectTagNamesForProject(project *models.Project) []string {
	return tagNamesFromInfos(BuildProjectTagList(project, false))
}

// CollectContextNamesForProject returns the sorted unique task contexts used in
// a project. Context has no project catalog, so task usage is its full vocabulary.
func CollectContextNamesForProject(project *models.Project) []string {
	if project == nil {
		return nil
	}
	contexts := make(map[string]bool)
	for _, task := range project.Tasks {
		if task == nil || task.Context == "" {
			continue
		}
		normalized, err := models.NormalizeTaskContext(task.Context)
		if err == nil {
			contexts[normalized] = true
		}
	}
	result := maps.Keys(contexts)
	return slices.Sorted(result)
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
	return operationalTitleBadge(FilterBadgeStyle, "Filters: "+strings.Join(parts, " "))
}

// newFilterState returns an initialized CommonFilterState with allocated maps.
func newFilterState() CommonFilterState {
	return CommonFilterState{
		Tags:  make(map[string]bool),
		Stale: false,
	}
}
