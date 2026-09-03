package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// newFilterDuePicker is the f@ single-select picker over the five due-token
// shortcuts, seeded from the active facet when one is set. Every row shows the
// token it writes and hands that token back, so the picker holds no vocabulary
// the grammar does not already carry, and the seeding compares canonical
// spellings rather than the filter cache's enums.
func (m *AppModel) newFilterDuePicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	shortcuts := query.DueFilterShortcuts()
	items := make([]ListItem, 0, len(shortcuts))
	byToken := make(map[string]ListItemID, len(shortcuts))
	for _, shortcut := range shortcuts {
		id := ListItemID("filter-due:" + shortcut.ID)
		token := shortcut.Token()
		byToken[token] = id
		items = append(items, ListItem{
			ID:            id,
			PrimaryText:   shortcut.Label,
			SecondaryText: token + " · " + shortcut.Secondary,
			Aliases:       []string{shortcut.ID, shortcut.Label, shortcut.Secondary, token},
			Data:          token,
		})
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		active := query.DueFilterClauses(m.crossOutliner.filter.DueDate)
		if m.crossOutliner.filter.Deferred {
			active = query.FilterClauseList(models.QueryFieldDeferred, models.QueryOperatorPresent, "")
		}
		if id, ok := byToken[strings.Join(query.FilterClauseParts(active), " ")]; ok {
			selected = []ListItemID{id}
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterDue,
		"Filter by Due or Deferred",
		"keep matching tasks",
		ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterDependencyPicker is the f~ single-select picker over the two
// reference directions, seeded from the active facet when one is set. It is
// built the same way the f@ picker is, from rows that are their own canonical
// tokens.
func (m *AppModel) newFilterDependencyPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	shortcuts := query.DependencyFilterShortcuts()
	items := make([]ListItem, 0, len(shortcuts))
	byToken := make(map[string]ListItemID, len(shortcuts))
	for _, shortcut := range shortcuts {
		id := ListItemID("filter-dependency:" + shortcut.ID)
		token := shortcut.Token()
		byToken[token] = id
		items = append(items, ListItem{
			ID:            id,
			PrimaryText:   shortcut.Label,
			SecondaryText: token + " · " + shortcut.Secondary,
			Aliases:       []string{shortcut.ID, shortcut.Label, shortcut.Secondary, token},
			Data:          token,
		})
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		active := query.ClausesForField(m.crossOutliner.filterQuery, models.QueryFieldBlocked)
		active = append(active,
			query.ClausesForField(m.crossOutliner.filterQuery, models.QueryFieldBlocking)...)
		if id, ok := byToken[strings.Join(query.FilterClauseParts(active), " ")]; ok {
			selected = []ListItemID{id}
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterDependency,
		"Filter by Dependency",
		"keep matching tasks",
		ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterHierarchyPicker is the fh single-select picker over the four
// hierarchy positions, seeded from the active facet when one is set.
func (m *AppModel) newFilterHierarchyPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	options := []struct {
		id        string
		label     string
		secondary string
		value     query.HierarchyFilter
	}{
		{id: "parent", label: "Parent", secondary: "has open children", value: query.HierarchyParent},
		{id: "child", label: "Child", secondary: "sits under a parent", value: query.HierarchyChild},
		{id: "root", label: "Root", secondary: "has no parent", value: query.HierarchyRoot},
		{id: "leaf", label: "Leaf", secondary: "has no open children", value: query.HierarchyLeaf},
	}
	items := make([]ListItem, 0, len(options))
	for _, option := range options {
		items = append(items, ListItem{
			ID:            ListItemID("filter-hierarchy:" + option.id),
			PrimaryText:   option.label,
			SecondaryText: option.secondary,
			Aliases:       []string{option.id, option.label, option.secondary},
			Data:          option.value,
		})
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		switch m.crossOutliner.filter.Hierarchy {
		case query.HierarchyAll:
			selected = nil
		case query.HierarchyParent:
			selected = []ListItemID{"filter-hierarchy:parent"}
		case query.HierarchyChild:
			selected = []ListItemID{"filter-hierarchy:child"}
		case query.HierarchyRoot:
			selected = []ListItemID{"filter-hierarchy:root"}
		case query.HierarchyLeaf:
			selected = []ListItemID{"filter-hierarchy:leaf"}
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterHierarchy,
		"Filter by Hierarchy",
		"keep matching tasks",
		ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterCompletionAgePicker is the fc single-select picker. Applying a
// completion-age filter also selects both terminal workflow states.
func (m *AppModel) newFilterCompletionAgePicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	options := []struct {
		id        string
		label     string
		secondary string
		value     query.CompletionAgeFilter
	}{
		{id: "all", label: "All ages", secondary: "any valid completion date", value: query.CompletionAgeAll},
		{id: "7", label: "7+ days", secondary: "completed at least 7 local days ago", value: query.CompletionAge7Days},
		{id: "30", label: "30+ days", secondary: "completed at least 30 local days ago", value: query.CompletionAge30Days},
		{id: "90", label: "90+ days", secondary: "completed at least 90 local days ago", value: query.CompletionAge90Days},
	}
	items := make([]ListItem, 0, len(options))
	for _, option := range options {
		items = append(items, ListItem{
			ID:            ListItemID("filter-completion-age:" + option.id),
			PrimaryText:   option.label,
			SecondaryText: option.secondary,
			Aliases:       []string{option.id, option.label, option.secondary},
			Data:          option.value,
		})
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil && m.crossOutliner.filter.HasCompletionAgeFilter() {
		suffix := "all"
		switch m.crossOutliner.filter.CompletionAge {
		case query.CompletionAge7Days:
			suffix = "7"
		case query.CompletionAge30Days:
			suffix = "30"
		case query.CompletionAge90Days:
			suffix = "90"
		default:
		}
		selected = []ListItemID{ListItemID("filter-completion-age:" + suffix)}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterCompletionAge,
		"Filter by Completion Age",
		"includes done and canceled tasks",
		ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterStatesPicker is the f$ multi-select picker over the canonical
// workflow states, seeded from the active state filter facet.
func (m *AppModel) newFilterStatesPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	source := ListSourceFromWorkflowStates(models.AllWorkflowStates())
	counts := m.prospectiveStateFilterCounts()
	for index := range source.Items {
		if state, ok := source.Items[index].Data.(models.WorkflowState); ok {
			source.Items[index].SecondaryText += " " + renderProspectiveFilterCount(counts[state])
			source.Items[index].Data = state.String()
		}
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		for _, state := range m.crossOutliner.filter.States.Values() {
			selected = append(selected, ListItemID("state:"+state.String()))
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterStates,
		"Filter by State",
		"keep matching tasks",
		source,
		true,
		selected,
	)
}

// prospectiveStateFilterCounts returns the matching-task buckets available to
// the state facet. The existing state selection is replaced because workflow
// states compose with OR; every other active facet, search, mode,
// Context/project scope, and focus remain in force.
func (m *AppModel) prospectiveStateFilterCounts() map[models.WorkflowState]int {
	counts := make(map[models.WorkflowState]int, len(models.AllWorkflowStates()))
	if m == nil || m.crossOutliner == nil {
		return counts
	}

	clauses := make([]models.QueryClause, 0, len(models.AllWorkflowStates()))
	for _, state := range models.AllWorkflowStates() {
		clauses = query.AppendFilterClause(clauses, models.QueryFieldState,
			models.QueryOperatorIs, state.String())
	}
	prospective := query.WithFields(m.crossOutliner.filterQuery,
		[]models.QueryField{models.QueryFieldState}, clauses)
	return prospectiveOutlinerStateCounts(m.crossOutliner, prospective)
}

// prospectiveOutlinerStateCounts runs the canonical row projection on an
// isolated model copy. Expanding task trees and sections keeps presentation
// folds from changing facet totals, while rebuilding through the normal mode
// pipeline preserves all actual scope and visibility rules.
func prospectiveOutlinerStateCounts(
	outliner *OutlinerModel,
	prospective models.Query,
) map[models.WorkflowState]int {
	counts := make(map[models.WorkflowState]int, len(models.AllWorkflowStates()))
	if outliner == nil {
		return counts
	}

	projection := *outliner
	// The copy takes the prospective narrowing through the one writer, so its
	// carriers are derived from the query the count is asked about rather than
	// the one the Outliner is still showing.
	projection.setFilterQuery(prospective)
	projection.rows = nil
	projection.OpenAtTask = nil
	projection.expandedNodes = make(map[taskref.TaskIdentity]bool)
	projection.collapsedSections = make(map[string]bool)
	projection.expandedChecklists = make(map[taskref.TaskIdentity]bool)
	projection.focusStack = append([]FocusEntry(nil), outliner.focusStack...)
	projection.crossFocusStack = append(
		[]CrossProjectFocusEntry(nil),
		outliner.crossFocusStack...,
	)
	projection.crossForceRebuild = true
	projection.rebuildRows()

	// A matching descendant can retain a non-matching ancestor as hierarchy
	// context. Count only rows that satisfy the prospective predicate themselves.
	// The facet half alone. Under a search projection the isolated copy carries
	// searchProjection != nil, so its rebuild yields fuzzy-scored rows that the
	// full work-mode predicate would then substring-gate with searchRemainder,
	// undercounting; facetPredicateAt is correct in both branches and applies the
	// facets exactly once.
	now := time.Now()
	matches := projection.facetPredicateAt(now)
	seen := make(map[taskref.TaskIdentity]struct{})
	for _, row := range projection.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		project := projection.resolveProjectForRow(row)
		if !matches(row.task, project) {
			continue
		}
		projectID := row.ProjectID
		if projectID == "" && project != nil {
			projectID = project.ID
		}
		identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: row.task.ID}
		if _, duplicate := seen[identity]; duplicate {
			continue
		}
		seen[identity] = struct{}{}
		counts[row.task.State()]++
	}
	return counts
}

func renderProspectiveFilterCount(count int) string {
	if count == 0 {
		zero := lipgloss.NewStyle().
			Foreground(theme.ColorError).
			Faint(true).
			Render("0")
		return "⋅ " + zero
	}
	return fmt.Sprintf("⋅ %d", count)
}

// newFilterTagsPicker is the f# multi-select picker over the active scope's tag
// vocabulary, seeded from the active tag filter facet. Contexts are a separate
// axis on fc, so the census here reads tags alone and the picker's CONTEXTS
// section drops out with them.
func (m *AppModel) newFilterTagsPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	tagList := query.BuildProjectsTagList(m.filterScopeProjects(), false)
	source := taskTagListSource(tagList, false)
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		for tag := range m.crossOutliner.filter.Tags {
			selected = append(selected, ListItemID("tag:"+tag))
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterTags,
		"Filter by Tags",
		"keep matching tasks",
		source,
		true,
		selected,
	)
}

// filterScopeProjects is the population a row facet's vocabulary is drawn from:
// the loaded scope once there is one, and the whole task workspace before the
// first load.
func (m *AppModel) filterScopeProjects() []*models.Project {
	if m.crossOutliner != nil && len(m.crossOutliner.loadedProjects) > 0 {
		return m.crossOutliner.workModeScopeProjects()
	}
	return m.taskWorkspaceProjects()
}

// newFilterContextsPicker is the fc multi-select picker over the active scope's
// context vocabulary, seeded from the active context filter facet. The rows
// compose with OR because a task carries at most one context, and the leading
// taskCountLabel is the census wording both context rows carry.
func taskCountLabel(count int) string {
	return fmt.Sprintf("%d %s", count, pluralize(count, "task", "tasks"))
}

// no-context row names the tasks the vocabulary does not cover.
func (m *AppModel) newFilterContextsPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	projects := m.filterScopeProjects()
	facets := query.BuildTagFacetList(projects, false)
	items := make([]ListItem, 0, len(facets)+1)
	items = append(items, ListItem{
		ID:            ListItemID("filter-context:" + query.NoContextFilterKey),
		PrimaryText:   "No context",
		SecondaryText: taskCountLabel(query.CountTasksWithoutContext(projects)),
		Aliases:       []string{"no context", "none", "##0"},
		Data:          query.NoContextFilterKey,
	})
	for _, facet := range facets {
		if facet.ContextCount == 0 {
			continue
		}
		items = append(items, ListItem{
			ID:            ListItemID("filter-context:" + facet.Name),
			PrimaryText:   "##" + facet.Name,
			SecondaryText: taskCountLabel(facet.ContextCount),
			Aliases:       []string{facet.Name, "##" + facet.Name},
			Data:          facet.Name,
		})
	}
	selected := []ListItemID(nil)
	if m.crossOutliner != nil {
		filter := &m.crossOutliner.filter.CommonFilterState
		if filter.NoContext {
			selected = append(selected, ListItemID("filter-context:"+query.NoContextFilterKey))
		}
		for name := range filter.Contexts {
			selected = append(selected, ListItemID("filter-context:"+name))
		}
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderFilterContexts,
		"Filter by Context",
		"keep matching tasks",
		ListSource{Items: items},
		true,
		selected,
	)
}
