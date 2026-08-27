package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// A named lens is a saved Outliner projection: the mode it opens, the query it
// narrows with, both filter families, and the manual sort it restores. This file
// owns the TUI-side representation of those axes and the only conversion between
// them and the durable config tokens, so no other TUI code reads a persisted
// spelling and no config code learns a TUI enum.

// lensState is the complete set of lens-owned Outliner axes. Scope is one of
// them, but it is the one axis a lens may decline to own: an empty
// scopeProjectID means the definition leaves the view's scope exactly as it
// found it, which is deliberately not the same as forcing the global
// population. Everything absent here belongs to the view the user is in rather
// than to the lens: folds, focus, zoom, selection, cursor, and viewport are all
// deliberately excluded.
type lensState struct {
	mode OutlinerModeID
	// scopeProjectID is the single project the lens reopens into, or empty when
	// the lens does not own the scope axis.
	scopeProjectID string
	query          string
	// filter carries the explicit activation flags along with the facet values,
	// because an active predicate that admits everything is not the same as an
	// absent one and only the flag can tell them apart.
	filter        FilterState
	projectFilter ProjectFilterState
	// sort is the remembered treeSort rather than the active render sort: a
	// fixed-order mode leaves the active sort at its default while the user's
	// manual choice survives in treeSort, and that choice is what the lens owns.
	sort SortState
}

// lensAxis maps one TUI enum onto its durable token set. The table is the single
// source of truth for that axis in both directions; the reverse lookup scans it
// rather than duplicating it into a second map that could drift out of step.
type lensAxis[T comparable] struct {
	name   string
	tokens map[T]string
	// fallback is the value the empty token decodes to. config writes the empty
	// string for an axis left at its default, so decoding must read it back as
	// that default rather than as garbage.
	fallback T
	// requiresToken marks the one axis, mode, that has no default rendering: an
	// empty token there is malformed rather than "left alone".
	requiresToken bool
}

func (a lensAxis[T]) token(value T) string {
	if token, ok := a.tokens[value]; ok {
		return token
	}
	if a.requiresToken {
		return ""
	}
	return a.tokens[a.fallback]
}

func (a lensAxis[T]) value(token string) (T, error) {
	if token == "" && !a.requiresToken {
		return a.fallback, nil
	}
	for value, candidate := range a.tokens {
		if candidate == token {
			return value, nil
		}
	}
	var zero T
	return zero, fmt.Errorf("lens %s %q is not a known value", a.name, token)
}

var lensModeAxis = lensAxis[OutlinerModeID]{
	name: "mode",
	tokens: map[OutlinerModeID]string{
		ModeAll:     config.LensModeAll,
		ModeInbox:   config.LensModeInbox,
		ModeToday:   config.LensModeToday,
		ModeNext:    config.LensModeNext,
		ModeBacklog: config.LensModeBacklog,
		ModeDue:     config.LensModeDue,
		ModeRecent:  config.LensModeRecent,
	},
	fallback:      ModeAll,
	requiresToken: true,
}

var lensSortAxis = lensAxis[SortField]{
	name: "sort field",
	tokens: map[SortField]string{
		SortByDefault:   config.LensSortDefault,
		SortByPriority:  config.LensSortPriority,
		SortByDueDate:   config.LensSortDue,
		SortByCreatedAt: config.LensSortCreated,
		SortByStatus:    config.LensSortStatus,
		SortByUpdatedAt: config.LensSortUpdated,
		SortByTitle:     config.LensSortTitle,
	},
	fallback: SortByDefault,
}

var lensDueAxis = lensAxis[DueDateFilter]{
	name: "due bucket",
	tokens: map[DueDateFilter]string{
		DueDateAll:      config.LensDueAll,
		DueDateOverdue:  config.LensDueOverdue,
		DueDateToday:    config.LensDueToday,
		DueDateThisWeek: config.LensDueWeek,
		DueDateNone:     config.LensDueNone,
	},
	fallback: DueDateAll,
}

var lensHierarchyAxis = lensAxis[HierarchyFilter]{
	name: "hierarchy",
	tokens: map[HierarchyFilter]string{
		HierarchyAll:    config.LensHierarchyAll,
		HierarchyParent: config.LensHierarchyParent,
		HierarchyChild:  config.LensHierarchyChild,
		HierarchyRoot:   config.LensHierarchyRoot,
		HierarchyLeaf:   config.LensHierarchyLeaf,
	},
	fallback: HierarchyAll,
}

var lensCompletionAgeAxis = lensAxis[CompletionAgeFilter]{
	name: "completion age",
	tokens: map[CompletionAgeFilter]string{
		CompletionAgeAll:    config.LensCompletionAgeAll,
		CompletionAge7Days:  config.LensCompletionAge7Days,
		CompletionAge30Days: config.LensCompletionAge30Days,
		CompletionAge90Days: config.LensCompletionAge90Days,
	},
	fallback: CompletionAgeAll,
}

var lensPopulationAxis = lensAxis[projectPopulationMode]{
	name: "population",
	tokens: map[projectPopulationMode]string{
		projectPopulationExclude: config.LensPopulationExclude,
		projectPopulationInclude: config.LensPopulationInclude,
		projectPopulationOnly:    config.LensPopulationOnly,
	},
	fallback: projectPopulationExclude,
}

// captureLensState reads every lens-owned axis off the live Outliner. The
// collections are detached so a later facet edit cannot reach back into a
// captured definition.
func (m *OutlinerModel) captureLensState() lensState {
	if m == nil {
		return lensState{}
	}
	return lensState{
		mode:           m.primaryMode,
		scopeProjectID: m.capturedLensScope(),
		query:          m.capturedLensQuery(),
		filter:         copyFilterState(m.filter),
		projectFilter:  m.projectFilter.clone(),
		sort:           m.treeSort,
	}
}

// capturedLensScope is the project scope a lens saved here can reopen into. The
// single-project Outliner is a scope no lens installs, and a parked focus is an
// exception to the base workspace rather than a scope the base Outliner can be
// asked for, so both are captured as no scope at all rather than as one every
// later application would have to refuse.
func (m *OutlinerModel) capturedLensScope() string {
	if m == nil || m.project != nil {
		return ""
	}
	if !m.baseWorkspaceAdmitsProject(m.scopedProjectID) {
		return ""
	}
	return m.scopedProjectID
}

// capturedLensQuery is the accepted query the current view is narrowed by,
// whichever facility owns it. Under a lens that is the lens's own narrowing
// query. Outside one an accepted ordinary-search projection owns the visible
// rows, so saving records the query that produced them rather than an empty
// axis; a search input still open has accepted nothing and is not captured.
// Only the query axis crosses over: the projection itself still cannot survive
// underneath a lens, and publishLens drops it in favor of the baseline that
// hands it back on the way out.
func (m *OutlinerModel) capturedLensQuery() string {
	if !m.hasActiveLens() && m.hasSearchProjection() && !m.IsSearchMode() {
		return m.SearchQuery()
	}
	return m.lensQuery
}

// lensTreeOrderPrev is the synthetic mode a lens is applied "out of". A lens
// stores its manual sort the way a tree-order mode holds it, so installing one
// is exactly the reconciliation ApplyOutlinerModeTransition performs when
// leaving a tree-order mode: manual-sort modes take the sort as their active
// order, and fixed-order modes keep their own while remembering it in treeSort.
var lensTreeOrderPrev = OutlinerMode{OrderPolicy: OrderPolicyTree}

// applyLensState installs a saved projection, reporting whether it named a
// catalog mode. Only the lens-owned axes are written; folds, focus, zoom,
// selection, and cursor belong to the view the user is in. The scope is written
// through pushProjectScope rather than by assigning the field, so the frame one
// Esc pops to is captured by the same helper every other scope push uses; a lens
// that names no scope leaves the current one alone. The push comes first,
// before a lens-owned axis is written, so the frame it captures holds the view
// the user was in rather than the projection being installed over it.
func (m *OutlinerModel) applyLensState(state lensState) bool {
	mode, ok := OutlinerModeByID(state.mode)
	if m == nil || !ok {
		return false
	}
	if state.scopeProjectID != "" {
		m.pushProjectScope(state.scopeProjectID)
	}
	m.primaryMode = state.mode
	m.filter = copyFilterState(state.filter)
	m.projectFilter = state.projectFilter.clone()
	m.setLensQuery(state.query)

	next := ApplyOutlinerModeTransition(
		OutlinerModeSwitch{TreeSort: state.sort, ActiveSort: state.sort},
		lensTreeOrderPrev,
		mode,
	)
	m.sort = next.ActiveSort
	m.treeSort = next.TreeSort
	return true
}

// namedLens renders the state as a durable definition under the given identity.
// Every axis is written with its canonical token rather than the empty
// omitted-default rendering, so one state has exactly one spelling here; the
// decoder still accepts the empty form config documents as that axis's default.
// Comparison therefore belongs to fingerprint, never to two DTOs.
func (s lensState) namedLens(id, name string) config.NamedLens {
	return config.NamedLens{
		ID:   id,
		Name: name,
		Mode: lensModeAxis.token(s.mode),
		// Scope has no token table: it is a project ID, and the empty string is
		// the unowned axis rather than a default the decoder has to name.
		Scope: s.scopeProjectID,
		Query: s.query,
		Tasks: config.LensTaskFilters{
			States:           lensWorkflowStateTokens(s.filter.States),
			StatesSet:        s.filter.hasStateFilter(),
			Priority:         s.filter.Priority,
			Tags:             lensSortedTagNames(s.filter.Tags),
			Contexts:         lensSortedTagNames(s.filter.Contexts),
			NoContext:        s.filter.NoContext,
			DueDate:          lensDueAxis.token(s.filter.DueDate),
			CompletionAge:    lensCompletionAgeAxis.token(s.filter.CompletionAge),
			CompletionAgeSet: s.filter.hasCompletionAgeFilter(),
			Hierarchy:        lensHierarchyAxis.token(s.filter.Hierarchy),
			Stale:            s.filter.Stale,
			Deferred:         s.filter.Deferred,
			Pinned:           s.filter.Pinned,
		},
		Projects: config.LensProjectFilters{
			Tags:         lensSortedTagNames(s.projectFilter.Tags),
			Untagged:     s.projectFilter.Untagged,
			ParkedMode:   lensPopulationAxis.token(s.projectFilter.ParkedMode),
			ArchivedMode: lensPopulationAxis.token(s.projectFilter.ArchivedMode),
		},
		Sort: config.LensSort{
			Field:     lensSortAxis.token(s.sort.Field),
			Ascending: s.sort.Ascending,
		},
	}
}

// lensStateFromNamedLens decodes a durable definition. The whole record is
// validated first, so a hand-edited machine file refuses here rather than
// installing a projection nobody can name. The query is deliberately not parsed:
// it compiles at projection time, where an unparseable one keeps the ordinary
// invalid-query guidance instead of failing to load.
func lensStateFromNamedLens(lens config.NamedLens) (lensState, error) {
	if err := lens.Validate(); err != nil {
		return lensState{}, err
	}
	mode, err := lensModeAxis.value(lens.Mode)
	if err != nil {
		return lensState{}, err
	}
	filter, err := lensFilterState(lens.Tasks)
	if err != nil {
		return lensState{}, err
	}
	projectFilter, err := lensProjectFilterState(lens.Projects)
	if err != nil {
		return lensState{}, err
	}
	sortField, err := lensSortAxis.value(lens.Sort.Field)
	if err != nil {
		return lensState{}, err
	}
	return lensState{
		mode: mode,
		// The scope is carried verbatim: Validate already held it to the canonical
		// UUID spelling, and whether that project can still be entered is a runtime
		// question ApplyNamedLens answers, never a decoding one.
		scopeProjectID: lens.Scope,
		query:          lens.Query,
		filter:         filter,
		projectFilter:  projectFilter,
		sort:           SortState{Field: sortField, Ascending: lens.Sort.Ascending},
	}, nil
}

func lensFilterState(tasks config.LensTaskFilters) (FilterState, error) {
	filter := NewFilterState()
	states := make([]models.WorkflowState, 0, len(tasks.States))
	for _, raw := range tasks.States {
		states = append(states, models.WorkflowState(raw))
	}
	set, err := models.NewWorkflowStateSet(states...)
	if err != nil {
		return FilterState{}, err
	}
	filter.setStateFilter(set, tasks.StatesSet)
	filter.Priority = tasks.Priority
	filter.Tags = lensTagSet(tasks.Tags)
	filter.Contexts = lensTagSet(tasks.Contexts)
	filter.NoContext = tasks.NoContext

	due, err := lensDueAxis.value(tasks.DueDate)
	if err != nil {
		return FilterState{}, err
	}
	filter.DueDate = due

	age, err := lensCompletionAgeAxis.value(tasks.CompletionAge)
	if err != nil {
		return FilterState{}, err
	}
	// The threshold is restored either way; only the activation flag decides
	// whether the cleanup facet is a live predicate.
	filter.CompletionAge = age
	if tasks.CompletionAgeSet {
		filter.setCompletionAgeFilter(age)
	}

	hierarchy, err := lensHierarchyAxis.value(tasks.Hierarchy)
	if err != nil {
		return FilterState{}, err
	}
	filter.Hierarchy = hierarchy
	filter.Stale = tasks.Stale
	filter.Deferred = tasks.Deferred
	filter.Pinned = tasks.Pinned
	return filter, nil
}

func lensProjectFilterState(projects config.LensProjectFilters) (ProjectFilterState, error) {
	filter := newProjectFilterState()
	filter.Tags = lensTagSet(projects.Tags)
	filter.Untagged = projects.Untagged
	parked, err := lensPopulationAxis.value(projects.ParkedMode)
	if err != nil {
		return ProjectFilterState{}, err
	}
	archived, err := lensPopulationAxis.value(projects.ArchivedMode)
	if err != nil {
		return ProjectFilterState{}, err
	}
	filter.ParkedMode = parked
	filter.ArchivedMode = archived
	return filter, nil
}

// fingerprint is the canonical rendering of one state. Every collection is
// emitted in a fixed order — workflow states in canonical lifecycle order, tags
// and contexts sorted — so map iteration can never make an unchanged lens read
// as modified. Every axis namedLens writes must appear here: an omitted one is
// silently unwatched, so an edit to it leaves the lens reading as unmodified.
func (s lensState) fingerprint() string {
	lens := s.namedLens("", "")
	return strings.Join([]string{
		lens.Mode,
		lens.Scope,
		lens.Query,
		strings.Join(lens.Tasks.States, ","),
		strconv.FormatBool(lens.Tasks.StatesSet),
		strconv.Itoa(lens.Tasks.Priority),
		strings.Join(lens.Tasks.Tags, ","),
		strings.Join(lens.Tasks.Contexts, ","),
		strconv.FormatBool(lens.Tasks.NoContext),
		lens.Tasks.DueDate,
		lens.Tasks.CompletionAge,
		strconv.FormatBool(lens.Tasks.CompletionAgeSet),
		lens.Tasks.Hierarchy,
		strconv.FormatBool(lens.Tasks.Stale),
		strconv.FormatBool(lens.Tasks.Deferred),
		strconv.FormatBool(lens.Tasks.Pinned),
		strings.Join(lens.Projects.Tags, ","),
		strconv.FormatBool(lens.Projects.Untagged),
		lens.Projects.ParkedMode,
		lens.Projects.ArchivedMode,
		lens.Sort.Field,
		strconv.FormatBool(lens.Sort.Ascending),
	}, "\x1f")
}

func lensWorkflowStateTokens(states models.WorkflowStateSet) []string {
	values := states.Values()
	if len(values) == 0 {
		return nil
	}
	tokens := make([]string, len(values))
	for i, state := range values {
		tokens[i] = string(state)
	}
	return tokens
}

func lensSortedTagNames(tags map[string]bool) []string {
	names := make([]string, 0, len(tags))
	for name, selected := range tags {
		if selected {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	slices.Sort(names)
	return names
}

func lensTagSet(names []string) map[string]bool {
	tags := make(map[string]bool, len(names))
	for _, name := range names {
		tags[name] = true
	}
	return tags
}
