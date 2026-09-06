package query

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// NoContextFilterKey is the pseudo-entry the context picker offers for tasks
// carrying no context at all. The empty context is legal, so without it a
// context filter could never name the tasks the vocabulary does not cover. It
// travels in the picker's value channel only; the state is the NoContext bool.
const NoContextFilterKey = "\x00nocontext"

// CommonFilterState contains filter fields shared across views.
type CommonFilterState struct {
	States   models.WorkflowStateSet // filter by one or more canonical workflow states
	Priority int                     // 0=all, N>0 keeps tasks whose effective priority is <= N
	Tags     map[string]bool         // filter by tag names (AND logic)

	// Contexts filters by task context name. The names compose with OR, unlike
	// the AND semantics of the tag filter, because a task carries at most one
	// context and an AND over two names could never match anything.
	Contexts map[string]bool

	// NoContext admits tasks whose optional context field is unset. It composes
	// with Contexts by OR, so the facet can name both a vocabulary and its gap.
	NoContext bool

	DueDate       DueDateFilter       // filter by due date
	CompletionAge CompletionAgeFilter // local-calendar age threshold for completed tasks
	Hierarchy     HierarchyFilter     // filter by hierarchy position
	Dependency    DependencyFilter    // filter by soft-reference position
	Stale         bool                // only show open work untouched for at least 90 local days
	Deferred      bool                // only show currently deferred tasks
	Pinned        bool                // only show pinned tasks
	// Review admits only tasks carrying the stored review flag. It is its own
	// facet rather than a tag name, because review is a boolean beside the pin
	// and no longer a value the tag vocabulary can mint or misspell.
	Review bool

	// statesSet distinguishes no state predicate from an explicit empty
	// match-nothing set produced by intersecting contradictory legacy filters.
	statesSet bool
	// completionAgeSet distinguishes an inactive cleanup facet from the active
	// "all ages" preset, which still excludes invalid CompletedAt values.
	completionAgeSet bool
}

// Nothing copies this carrier. It is derived from the canonical query by
// setFilterQuery and by nothing else, so the way to hold onto a narrowing is to
// hold the query — which is what a restoration point does, and the only form
// that can carry a clause's polarity and both dependency directions back.

func cloneCommonWorkflowStateSet(states models.WorkflowStateSet) models.WorkflowStateSet {
	cloned, err := models.NewWorkflowStateSet(states.Values()...)
	if err != nil {
		panic(err)
	}
	return cloned
}

func (f *CommonFilterState) HasStateFilter() bool {
	return f.statesSet || !f.States.IsEmpty()
}

// SetStateFilter always arms the filter, including for an empty set: an explicit
// empty selection is a state the caller chose, not the absence of one.
func (f *CommonFilterState) SetStateFilter(states models.WorkflowStateSet) {
	f.States = cloneCommonWorkflowStateSet(states)
	f.statesSet = true
}

func (f *CommonFilterState) HasCompletionAgeFilter() bool {
	return f.completionAgeSet
}

func (f *CommonFilterState) SetCompletionAgeFilter(filter CompletionAgeFilter) {
	f.CompletionAge = filter
	f.completionAgeSet = true
}

// The row predicate this cache stands for is models.Query.Admits, reached
// through OutlinerModel.rowPredicateQuery. There is no second evaluation of the
// facets here: a task is admitted by the clauses the cache was derived from, so
// a facet, a chip, and a saved lens cannot disagree about what a filter means.
//
// The hierarchy and dependency facets are the two axes Admits does not answer
// for the Outliner. Hierarchy needs the scope's open-direct-child census, and
// the task arriving at the predicate can be a promotion-pass clone whose
// Children were rewritten; evaluating it against the handed-over tree would make
// "parent" mean one thing in All and another in a flat mode. Dependency needs
// the loaded population's reference index, because dependents are computed
// rather than stored. passesHierarchyFilter and passesDependencyFilter answer
// them.

// HasActiveCommonFilters returns true if any common filter is non-default.
func HasActiveCommonFilters(f *CommonFilterState) bool {
	return f.HasStateFilter() ||
		f.HasCompletionAgeFilter() ||
		f.Priority != 0 ||
		len(f.Tags) > 0 ||
		len(f.Contexts) > 0 ||
		f.NoContext ||
		f.DueDate != DueDateAll ||
		f.Hierarchy != HierarchyAll ||
		f.Dependency != DependencyFilterAll ||
		f.Stale ||
		f.Deferred ||
		f.Pinned ||
		f.Review
}

// CommonFilterFacetCount reports independently clearable task facets. The
// terminal-state predicate installed by a completion-age choice is part of
// that one cleanup facet, not a second state facet in the clear count.
func CommonFilterFacetCount(f *CommonFilterState) int {
	if f == nil {
		return 0
	}
	count := 0
	if f.HasStateFilter() && !CompletionAgeOwnsTerminalStates(f) {
		count++
	}
	if f.HasCompletionAgeFilter() {
		count++
	}
	if f.Priority != 0 {
		count++
	}
	if len(f.Tags) > 0 {
		count++
	}
	// Context names and the no-context pseudo-entry share one picker and
	// therefore count once, the same way the project picker's descriptive tags
	// and its Untagged row do.
	if len(f.Contexts) > 0 || f.NoContext {
		count++
	}
	if f.DueDate != DueDateAll {
		count++
	}
	if f.Hierarchy != HierarchyAll {
		count++
	}
	if f.Dependency != DependencyFilterAll {
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
	if f.Review {
		count++
	}
	return count
}

func CompletionAgeOwnsTerminalStates(f *CommonFilterState) bool {
	if f == nil || !f.HasCompletionAgeFilter() || !f.HasStateFilter() {
		return false
	}
	terminal, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	return err == nil && f.States.Equal(terminal)
}

// CommonFilterParts returns the active row facets as chrome chips. The chips are
// the clauses' own canonical sigil text, so what the badge shows is spelled the
// way the same narrowing is typed into search and saved into a lens.
func CommonFilterParts(f *CommonFilterState) []string {
	return FilterClauseParts(CommonFilterClauses(f))
}

// CollectContextNamesForProjects returns the sorted context vocabulary in use
// across a set of projects. It reads the same BuildTagFacetList census the fc
// picker takes its counts from, so the names a picker offers and the counts
// beside them can never come from two different passes over the tasks.
func CollectContextNamesForProjects(projects []*models.Project) []string {
	names := make([]string, 0)
	for _, info := range BuildTagFacetList(projects, false) {
		if info.ContextCount > 0 {
			names = append(names, info.Name)
		}
	}
	return names
}

// CountTasksWithoutContext counts the gap the no-context row names, over the
// same population BuildTagFacetList reads for the rows beside it.
func CountTasksWithoutContext(projects []*models.Project) int {
	count := 0
	for _, project := range uniqueProjects(projects) {
		for _, task := range project.Tasks {
			if task != nil && task.Context == "" {
				count++
			}
		}
	}
	return count
}

// FilterState contains all task filtering options including view-specific fields
// beyond the common set. Used by ProjectTree and Outliner.
type FilterState struct {
	CommonFilterState // embedded common filter fields
}

// NewFilterState returns an initialized FilterState with allocated maps.
func NewFilterState() FilterState {
	return FilterState{
		CommonFilterState: NewCommonFilterState(),
	}
}

// RenderFilterBadge renders a styled filter badge from a parts list.
// Returns empty string if parts is empty.
func RenderFilterBadge(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return pulse.OperationalTitleBadge(theme.FilterBadgeStyle, "Filters: "+strings.Join(parts, " "))
}

// NewCommonFilterState returns an initialized CommonFilterState with allocated maps.
func NewCommonFilterState() CommonFilterState {
	return CommonFilterState{
		Tags:     make(map[string]bool),
		Contexts: make(map[string]bool),
		Stale:    false,
	}
}
