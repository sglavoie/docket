package outliner

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/util"
)

// rowFilterClauseEdit is one clause constructor per row facet. Each reads the
// picker's value and spells what its fields now say; none of them mutates
// anything, so the whole f chord has exactly one writer — applyFilterClauseEdit
// in the shell.
//
//nolint:gocyclo // One constructor per facet, kept together so the set is auditable.
func (m *OutlinerModel) RowFilterClauseEdit(invocation operation.OperationInvocation) (FilterClauseEdit, error) {
	switch invocation.Descriptor.ID {
	case operation.OperationFilterPriority:
		threshold, ok := invocation.Value.(int)
		if !ok || threshold < 0 || threshold > models.PriorityMax {
			return FilterClauseEdit{}, fmt.Errorf("invalid priority threshold")
		}
		if threshold == 0 {
			return FilterClauseEdit{Status: "Priority filter cleared"}, nil
		}
		return FilterClauseEdit{
			Clauses: query.FilterClauseList(models.QueryFieldPriority,
				models.QueryOperatorAtLeast, strconv.Itoa(threshold)),
			Status: "Filtering " + priorityThresholdLabel(threshold),
		}, nil
	case operation.OperationFilterDue:
		token, ok := invocation.Value.(string)
		if !ok {
			return FilterClauseEdit{}, fmt.Errorf("invalid due-date filter value")
		}
		shortcut, known := query.DueFilterShortcutForToken(token)
		if !known {
			return FilterClauseEdit{}, fmt.Errorf("unknown due filter %q", token)
		}
		return FilterClauseEdit{
			Clauses: shortcut.Clauses,
			Status:  "Filtering " + token,
		}, nil
	case operation.OperationFilterCompletionAge:
		age, ok := invocation.Value.(query.CompletionAgeFilter)
		if !ok || age < query.CompletionAgeAll || age > query.CompletionAge90Days {
			return FilterClauseEdit{}, fmt.Errorf("invalid completion-age filter value")
		}
		return FilterClauseEdit{
			Clauses: query.CompletionAgeClauses(age),
			Status:  "Filtering completed: " + completionAgeFilterLabel(age),
		}, nil
	case operation.OperationFilterHierarchy:
		hierarchy, ok := invocation.Value.(query.HierarchyFilter)
		if !ok || hierarchy < query.HierarchyParent || hierarchy > query.HierarchyLeaf {
			return FilterClauseEdit{}, fmt.Errorf("invalid hierarchy filter value")
		}
		return FilterClauseEdit{
			Clauses: query.HierarchyFilterClauses(hierarchy),
			Status:  "Filtering hierarchy: " + HierarchyFilterLabel(hierarchy),
		}, nil
	case operation.OperationFilterDependency:
		token, ok := invocation.Value.(string)
		if !ok {
			return FilterClauseEdit{}, fmt.Errorf("invalid dependency filter value")
		}
		shortcut, known := query.DependencyFilterShortcutForToken(token)
		if !known {
			return FilterClauseEdit{}, fmt.Errorf("unknown dependency filter %q", token)
		}
		return FilterClauseEdit{
			Clauses: shortcut.Clauses,
			Status:  "Filtering " + token,
		}, nil
	case operation.OperationFilterStates:
		names, ok := invocation.Value.([]string)
		if !ok {
			return FilterClauseEdit{}, fmt.Errorf("invalid state filter value")
		}
		clauses := make([]models.QueryClause, 0, len(names))
		for _, name := range names {
			if !models.WorkflowState(name).IsValid() {
				return FilterClauseEdit{}, fmt.Errorf("invalid workflow state %q", name)
			}
			clauses = query.AppendFilterClause(clauses, models.QueryFieldState,
				models.QueryOperatorIs, name)
		}
		status := "Filtering " + util.Count(len(names), "state")
		if len(names) == 0 {
			status = "State filter cleared"
		}
		return FilterClauseEdit{Clauses: clauses, Status: status}, nil
	case operation.OperationFilterTags:
		names, ok := invocation.Value.([]string)
		if !ok {
			return FilterClauseEdit{}, fmt.Errorf("invalid tag filter value")
		}
		clauses := make([]models.QueryClause, 0, len(names))
		for _, name := range names {
			clauses = query.AppendFilterClause(clauses, models.QueryFieldTag,
				models.QueryOperatorIs, name)
		}
		status := "Filtering " + util.Count(len(names), "tag")
		if len(names) == 0 {
			status = "Tag filter cleared"
		}
		return FilterClauseEdit{Clauses: clauses, Status: status}, nil
	case operation.OperationFilterContexts:
		names, ok := invocation.Value.([]string)
		if !ok {
			return FilterClauseEdit{}, fmt.Errorf("invalid context filter value")
		}
		clauses := query.ContextFilterClauses(names)
		status := "Context filter cleared"
		if len(clauses) > 0 {
			status = "Filtering context: " + strings.Join(query.FilterClauseParts(clauses), " ")
		}
		return FilterClauseEdit{Clauses: clauses, Status: status}, nil
	case operation.OperationFilterProjects:
		slugs, ok := invocation.Value.([]string)
		if !ok {
			return FilterClauseEdit{}, fmt.Errorf("invalid project filter value")
		}
		clauses := query.ProjectFilterClauses(slugs)
		status := "Project filter cleared"
		if len(clauses) > 0 {
			status = "Filtering " + util.Count(len(clauses), "project")
		}
		return FilterClauseEdit{Clauses: clauses, Status: status}, nil
	case operation.OperationFilterReview:
		if m.Filter.Review {
			return FilterClauseEdit{Status: "Review filter cleared"}, nil
		}
		return FilterClauseEdit{
			Clauses: query.FilterClauseList(models.QueryFieldReview, models.QueryOperatorPresent, ""),
			Status:  "Filtering Review",
		}, nil
	case operation.OperationFilterStale:
		if m.Filter.Stale {
			return FilterClauseEdit{Status: "Stale filter cleared"}, nil
		}
		return FilterClauseEdit{
			Clauses: query.FilterClauseList(models.QueryFieldStale, models.QueryOperatorPresent, ""),
			Status:  "Filtering Stale",
		}, nil
	case operation.OperationFilterPinned:
		if m.Filter.Pinned {
			return FilterClauseEdit{Status: "Pinned filter cleared"}, nil
		}
		return FilterClauseEdit{
			Clauses: query.FilterClauseList(models.QueryFieldPinned, models.QueryOperatorPresent, ""),
			Status:  "Filtering Pinned",
		}, nil
	case operation.OperationFilterClear:
		return FilterClauseEdit{Status: "Filters cleared"}, nil
	default:
		return FilterClauseEdit{}, fmt.Errorf("unknown filter operation %s", invocation.Descriptor.ID)
	}
}

func priorityThresholdLabel(threshold int) string {
	switch threshold {
	case models.PriorityHigh:
		return "High priority"
	case models.PriorityMedium:
		return "Medium priority or higher"
	case models.PriorityLow:
		return "Low priority or higher"
	default:
		return "priority"
	}
}

func HierarchyFilterLabel(filter query.HierarchyFilter) string {
	switch filter {
	case query.HierarchyParent:
		return "parent"
	case query.HierarchyChild:
		return "child"
	case query.HierarchyRoot:
		return "root"
	case query.HierarchyLeaf:
		return "leaf"
	default:
		return "all"
	}
}

func completionAgeFilterLabel(filter query.CompletionAgeFilter) string {
	if days := filter.Days(); days > 0 {
		return fmt.Sprintf("at least %d days ago", days)
	}
	return "all ages"
}

// filterOperationFields names the clause fields one facet owns. A facet writes
// these fields outright and nothing else, so applying it can neither preserve a
// value it just replaced nor disturb a facet beside it.
func FilterOperationFields(id operation.OperationID) []models.QueryField {
	switch id {
	case operation.OperationFilterPriority:
		return []models.QueryField{models.QueryFieldPriority}
	case operation.OperationFilterStates:
		return []models.QueryField{models.QueryFieldState}
	case operation.OperationFilterTags:
		return []models.QueryField{models.QueryFieldTag}
	case operation.OperationFilterContexts:
		return []models.QueryField{models.QueryFieldContext}
	case operation.OperationFilterProjects:
		return []models.QueryField{models.QueryFieldProject}
	case operation.OperationFilterDue:
		// The due picker's deferred row and its four bucket rows are one facet:
		// choosing either clears the other, which is only true if both fields
		// belong to the same operation.
		return []models.QueryField{models.QueryFieldDue, models.QueryFieldDeferred}
	case operation.OperationFilterCompletionAge:
		// The cleanup facet installs the terminal-state predicate itself, which is
		// why CommonFilterFacetCount reads the pair as one clearable facet.
		return []models.QueryField{models.QueryFieldCompleted, models.QueryFieldState}
	case operation.OperationFilterHierarchy:
		return query.FilterQueryHierarchyFields
	case operation.OperationFilterDependency:
		// Both directions are one facet: choosing either clears the other, the
		// same way the due picker's buckets and its deferred row do.
		return query.FilterQueryDependencyFields
	case operation.OperationFilterStale:
		return []models.QueryField{models.QueryFieldStale}
	case operation.OperationFilterPinned:
		return []models.QueryField{models.QueryFieldPinned}
	case operation.OperationFilterReview:
		return []models.QueryField{models.QueryFieldReview}
	case operation.OperationFilterClear:
		return query.FilterQueryTaskFields()
	case operation.OperationFilterProjectTags:
		// The F# picker carries the parked row in the same list of names, so the
		// tag restriction and the parked axis move together.
		return []models.QueryField{models.QueryFieldProjectTag, models.QueryFieldParked}
	case operation.OperationFilterShowParked, operation.OperationFilterOnlyParked:
		return []models.QueryField{models.QueryFieldParked}
	case operation.OperationFilterIncludeArchived, operation.OperationFilterOnlyArchived:
		return []models.QueryField{models.QueryFieldArchived}
	case operation.OperationFilterProjectClear:
		return query.FilterQueryPopulationFields
	default:
		return nil
	}
}

// filterOperationRequiresReload reports whether applying a facet reloads the
// population rather than rebuilding the rows. It is computed from the fields the
// operation writes rather than declared beside its handler, so a facet cannot be
// registered claiming one obligation and implemented honoring another.
func FilterOperationRequiresReload(id operation.OperationID) bool {
	for _, field := range FilterOperationFields(id) {
		if models.QueryFieldRequiresReload(field) {
			return true
		}
	}
	return false
}

// filterClauseEdit is what one facet constructor produces: the clauses its own
// fields now carry, and the status the surface reports. Narrowing nothing is an
// empty clause list, not a separate clearing verb.
type FilterClauseEdit struct {
	Clauses []models.QueryClause
	Status  string
	// declined reports a facet that changed nothing and only has something to
	// say, such as a clear with nothing to clear.
	Declined bool
}

// populationFilterClauseEdit is one clause constructor per population facet.
// Each toggle reads the axis the derived cache currently holds and spells the
// mode the key selects, so Fp, FP, and the F# picker's parked row cannot leave
// the population claiming different things. Like the row facets, it mutates
// nothing: applyFilterClauseEdit in filter_operations.go is the one writer.
func (m *OutlinerModel) PopulationFilterClauseEdit(
	invocation operation.OperationInvocation,
) (FilterClauseEdit, error) {
	switch invocation.Descriptor.ID {
	case operation.OperationFilterProjectTags:
		names, ok := invocation.Value.([]string)
		if !ok {
			return FilterClauseEdit{}, fmt.Errorf("invalid project tag filter value")
		}
		next := m.ProjectFilter.Clone()
		query.ApplyProjectTagFilterValue(&next, names)
		clauses := append(query.ProjectTagClauses(&next),
			query.PopulationClauses(models.QueryFieldParked, next.ParkedMode)...)
		return FilterClauseEdit{Clauses: clauses, Status: query.ProjectTagFilterStatus(&next)}, nil
	case operation.OperationFilterShowParked:
		return m.populationToggleEdit(models.QueryFieldParked, m.ProjectFilter.ParkedMode,
			query.ProjectPopulationInclude, "Parked"), nil
	case operation.OperationFilterOnlyParked:
		return m.populationToggleEdit(models.QueryFieldParked, m.ProjectFilter.ParkedMode,
			query.ProjectPopulationOnly, "Parked"), nil
	case operation.OperationFilterIncludeArchived:
		return m.populationToggleEdit(models.QueryFieldArchived, m.ProjectFilter.ArchivedMode,
			query.ProjectPopulationInclude, "Archived"), nil
	case operation.OperationFilterOnlyArchived:
		return m.populationToggleEdit(models.QueryFieldArchived, m.ProjectFilter.ArchivedMode,
			query.ProjectPopulationOnly, "Archived"), nil
	case operation.OperationFilterProjectClear:
		if !query.HasActiveProjectFilters(&m.ProjectFilter) {
			return FilterClauseEdit{Declined: true, Status: "No project filters are active"}, nil
		}
		return FilterClauseEdit{Status: "Project filters cleared"}, nil
	default:
		return FilterClauseEdit{}, fmt.Errorf(
			"unknown project filter operation %s", invocation.Descriptor.ID)
	}
}

func (m *OutlinerModel) populationToggleEdit(
	field models.QueryField,
	current, target query.ProjectPopulationMode,
	name string,
) FilterClauseEdit {
	mode := query.ToggleProjectPopulation(current, target)
	return FilterClauseEdit{
		Clauses: query.PopulationClauses(field, mode),
		Status:  projectPopulationStatus(mode, name),
	}
}

// projectPopulationStatus names the population one axis now loads rather than
// the key that was pressed, because two keys reach the same three modes.
func projectPopulationStatus(mode query.ProjectPopulationMode, name string) string {
	switch mode {
	case query.ProjectPopulationOnly:
		return "Showing only " + strings.ToLower(name) + " projects"
	case query.ProjectPopulationInclude:
		return "Showing " + strings.ToLower(name) + " projects"
	default:
		return name + " projects hidden"
	}
}

// The Outliner half of the filter query state: the model owns exactly one
// narrowing predicate, and these are the reads and writes that need the model to
// answer. The derivations they call — the carrier caches, the field families,
// and the clause arithmetic — are pure and live below, so what stays here is the
// part that could not.

// setFilterQuery installs the narrowing predicate and rewrites both caches from
// it. It is the only production writer of either carrier.
func (m *OutlinerModel) SetFilterQuery(narrowing models.Query) {
	if m == nil {
		return
	}
	m.FilterQuery = narrowing.Canonical()
	m.Filter.CommonFilterState = query.CommonFilterStateFromQuery(m.FilterQuery)
	m.ProjectFilter = query.ProjectFilterStateFromQuery(m.FilterQuery)
}

// replaceFilterClauses writes one facet's fields and re-derives.
func (m *OutlinerModel) ReplaceFilterClauses(
	fields []models.QueryField,
	clauses []models.QueryClause,
) {
	if m == nil {
		return
	}
	m.SetFilterQuery(query.WithFields(m.FilterQuery, fields, clauses))
}

// A restoration point carries the predicate itself rather than the carrier
// caches derived from it, so the two writes below are how a captured predicate
// gets back in. Both go through setFilterQuery, which re-derives both caches, so
// a restore can never leave a picker, a chip, and the row predicate reading
// different answers.
//
// They are two writes rather than one because the predicate has two halves with
// different owners. The row and census clauses describe the projection and are
// answered against whatever is loaded; the population clauses name which
// projects were read at all, and putting those back without the projects they
// selected would leave the query claiming a population nothing went and fetched.
// So the presentation half restores on its own, over the population in hand, and
// the population half restores only beside the projects it chose.

// restoreNarrowingPredicate installs a captured predicate's projection half and
// keeps the population half the live view is standing in. It is what a
// presentation restored over current data owes: the mode, facets, and ordering
// come back exactly as captured — polarity, dependency direction and all — while
// the loaded projects stay whatever the view last read.
func (m *OutlinerModel) restoreNarrowingPredicate(captured models.Query) {
	if m == nil {
		return
	}
	m.SetFilterQuery(query.WithFields(captured, query.FilterQueryPopulationFields,
		query.ClausesOfFields(m.FilterQuery, query.FilterQueryPopulationFields...)))
}

// adoptPopulationNarrowing writes a captured predicate's population half over
// the live one and leaves every projection clause alone. Its one caller restores
// the projects those clauses selected in the same breath, which is what makes
// the pair honest: the facets and the population they chose are one fact.
func (m *OutlinerModel) adoptPopulationNarrowing(captured models.Query) {
	if m == nil {
		return
	}
	m.SetFilterQuery(query.WithFields(m.FilterQuery, query.FilterQueryPopulationFields,
		query.ClausesOfFields(captured, query.FilterQueryPopulationFields...)))
}

// rowPredicateQuery is the half of the filter query models.Query.Admits answers.
// The population half is lifted out because a project this Outliner never loaded
// cannot be judged from a row, and the census half because its two censuses —
// the scope's open-direct-child count and the loaded population's reference
// index — live in passesHierarchyFilter and passesDependencyFilter.
func (m *OutlinerModel) rowPredicateQuery() models.Query {
	if m == nil {
		return models.Query{}
	}
	fields := append(slices.Clone(query.FilterQueryPopulationFields), query.FilterQueryCensusFields()...)
	return query.WithoutFields(m.FilterQuery, fields...)
}

// admitsTerminalPopulation is the one terminal-admission rule. Every surface
// that used to hold a rule of its own reads it instead: terminal work is on
// screen when the active query names it, and nothing else puts it there. The
// facet half is answered by the canonical filter query and the sigil half by the
// compiled narrowing gate, because "$done" typed into a search and Done chosen
// from the fs picker are the same clause said twice.
//
// Recent is the one projection that does not ask. It reports what happened, so
// it owns terminal and deferred membership outright rather than waiting for a
// query to open them.
func (m *OutlinerModel) AdmitsTerminalPopulation() bool {
	return m != nil &&
		(m.FilterQuery.OpensTerminalPopulation() || m.activeNarrowingGate().AllowsTerminal())
}

// admitsDeferredPopulation is that same rule read on the deferral axis.
func (m *OutlinerModel) admitsDeferredPopulation() bool {
	return m != nil &&
		(m.FilterQuery.OpensDeferredPopulation() || m.activeNarrowingGate().AllowsDeferred())
}

// AdmitsHiddenPopulation is that same rule read on the hidden axis. Only the
// filter query can open it, since no sigil spells it; a saved lens carries its
// hidden clause into that query with the rest of its facets.
func (m *OutlinerModel) AdmitsHiddenPopulation() bool {
	return m != nil && m.FilterQuery.OpensHiddenPopulation()
}

// activeNarrowingGate is the compiled query the current projection was built
// from: a lens's saved query when a lens owns the view, otherwise the search
// input's. Exactly one can be live, because slash search under a lens edits that
// lens's query rather than opening a projection of its own. A nil gate is a view
// with no narrowing query at all, and every reader here tolerates one.
func (m *OutlinerModel) activeNarrowingGate() *query.SearchPredicates {
	if m == nil {
		return nil
	}
	if m.HasActiveLens() {
		return m.Lens.Gate
	}
	return m.Search.Gate
}

// filterQueryProject resolves the project a row is read inside, falling back to
// the single-project Outliner's own project the way rowIdentity does.
func (m *OutlinerModel) filterQueryProject(project *models.Project) *models.Project {
	if project != nil {
		return project
	}
	if m != nil && m.Project != nil {
		return m.Project
	}
	return query.EmptyFilterQueryProject
}
