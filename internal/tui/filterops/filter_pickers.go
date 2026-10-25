package filterops

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/tui/valuepicker"
)

// newFilterDuePicker is the f@ single-select picker over the five due-token
// shortcuts, seeded from the active facet when one is set. Every row shows the
// token it writes and hands that token back, so the picker holds no vocabulary
// the grammar does not already carry, and the seeding compares canonical
// spellings rather than the filter cache's enums.
func (c *Controller) NewFilterDuePicker(shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	shortcuts := query.DueFilterShortcuts()
	items := make([]listsurface.ListItem, 0, len(shortcuts))
	byToken := make(map[string]listsurface.ListItemID, len(shortcuts))
	for _, shortcut := range shortcuts {
		id := listsurface.ListItemID("filter-due:" + shortcut.ID)
		token := shortcut.Token()
		byToken[token] = id
		items = append(items, listsurface.ListItem{
			ID:            id,
			PrimaryText:   shortcut.Label,
			SecondaryText: token + " · " + shortcut.Secondary,
			Aliases:       []string{shortcut.ID, shortcut.Label, shortcut.Secondary, token},
			Data:          token,
		})
	}
	selected := []listsurface.ListItemID(nil)
	if shell.Outliner() != nil {
		active := query.DueFilterClauses(shell.Outliner().Filter.DueDate)
		if shell.Outliner().Filter.Deferred {
			active = query.FilterClauseList(models.QueryFieldDeferred, models.QueryOperatorPresent, "")
		}
		if id, ok := byToken[strings.Join(query.FilterClauseParts(active), " ")]; ok {
			selected = []listsurface.ListItemID{id}
		}
	}
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderFilterDue,
		"Filter by Due or Deferred",
		"keep matching tasks",
		listsurface.ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterDependencyPicker is the f~ single-select picker over the two
// reference directions, seeded from the active facet when one is set. It is
// built the same way the f@ picker is, from rows that are their own canonical
// tokens.
func (c *Controller) NewFilterDependencyPicker(shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	shortcuts := query.DependencyFilterShortcuts()
	items := make([]listsurface.ListItem, 0, len(shortcuts))
	byToken := make(map[string]listsurface.ListItemID, len(shortcuts))
	for _, shortcut := range shortcuts {
		id := listsurface.ListItemID("filter-dependency:" + shortcut.ID)
		token := shortcut.Token()
		byToken[token] = id
		items = append(items, listsurface.ListItem{
			ID:            id,
			PrimaryText:   shortcut.Label,
			SecondaryText: token + " · " + shortcut.Secondary,
			Aliases:       []string{shortcut.ID, shortcut.Label, shortcut.Secondary, token},
			Data:          token,
		})
	}
	selected := []listsurface.ListItemID(nil)
	if shell.Outliner() != nil {
		active := query.ClausesForField(shell.Outliner().FilterQuery, models.QueryFieldBlocked)
		active = append(active,
			query.ClausesForField(shell.Outliner().FilterQuery, models.QueryFieldBlocking)...)
		if id, ok := byToken[strings.Join(query.FilterClauseParts(active), " ")]; ok {
			selected = []listsurface.ListItemID{id}
		}
	}
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderFilterDependency,
		"Filter by Dependency",
		"keep matching tasks",
		listsurface.ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterHierarchyPicker is the fh single-select picker over the four
// hierarchy positions, seeded from the active facet when one is set.
func (c *Controller) NewFilterHierarchyPicker(shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
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
	items := make([]listsurface.ListItem, 0, len(options))
	for _, option := range options {
		items = append(items, listsurface.ListItem{
			ID:            listsurface.ListItemID("filter-hierarchy:" + option.id),
			PrimaryText:   option.label,
			SecondaryText: option.secondary,
			Aliases:       []string{option.id, option.label, option.secondary},
			Data:          option.value,
		})
	}
	selected := []listsurface.ListItemID(nil)
	if shell.Outliner() != nil {
		switch shell.Outliner().Filter.Hierarchy {
		case query.HierarchyAll:
			selected = nil
		case query.HierarchyParent:
			selected = []listsurface.ListItemID{"filter-hierarchy:parent"}
		case query.HierarchyChild:
			selected = []listsurface.ListItemID{"filter-hierarchy:child"}
		case query.HierarchyRoot:
			selected = []listsurface.ListItemID{"filter-hierarchy:root"}
		case query.HierarchyLeaf:
			selected = []listsurface.ListItemID{"filter-hierarchy:leaf"}
		}
	}
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderFilterHierarchy,
		"Filter by Hierarchy",
		"keep matching tasks",
		listsurface.ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterCompletionAgePicker is the fc single-select picker. Applying a
// completion-age filter also selects both terminal workflow states.
func (c *Controller) NewFilterCompletionAgePicker(shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
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
	items := make([]listsurface.ListItem, 0, len(options))
	for _, option := range options {
		items = append(items, listsurface.ListItem{
			ID:            listsurface.ListItemID("filter-completion-age:" + option.id),
			PrimaryText:   option.label,
			SecondaryText: option.secondary,
			Aliases:       []string{option.id, option.label, option.secondary},
			Data:          option.value,
		})
	}
	selected := []listsurface.ListItemID(nil)
	if shell.Outliner() != nil && shell.Outliner().Filter.HasCompletionAgeFilter() {
		suffix := "all"
		switch shell.Outliner().Filter.CompletionAge {
		case query.CompletionAge7Days:
			suffix = "7"
		case query.CompletionAge30Days:
			suffix = "30"
		case query.CompletionAge90Days:
			suffix = "90"
		default:
		}
		selected = []listsurface.ListItemID{listsurface.ListItemID("filter-completion-age:" + suffix)}
	}
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderFilterCompletionAge,
		"Filter by Completion Age",
		"includes done and canceled tasks",
		listsurface.ListSource{Items: items},
		false,
		selected,
	)
}

// newFilterStatesPicker is the fw multi-select picker over the canonical
// workflow states, seeded from the active state filter facet.
func (c *Controller) NewFilterStatesPicker(shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	source := listsurface.ListSourceFromWorkflowStates(models.AllWorkflowStates())
	counts := c.ProspectiveStateFilterCounts(shell)
	for index := range source.Items {
		if state, ok := source.Items[index].Data.(models.WorkflowState); ok {
			source.Items[index].SecondaryText += " " + renderProspectiveFilterCount(counts[state])
			source.Items[index].Data = state.String()
		}
	}
	selected := []listsurface.ListItemID(nil)
	if shell.Outliner() != nil {
		for _, state := range shell.Outliner().Filter.States.Values() {
			selected = append(selected, listsurface.ListItemID("state:"+state.String()))
		}
	}
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderFilterStates,
		"Filter by Status",
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
func (c *Controller) ProspectiveStateFilterCounts(shell Shell) map[models.WorkflowState]int {
	counts := make(map[models.WorkflowState]int, len(models.AllWorkflowStates()))
	if c == nil || shell.Outliner() == nil {
		return counts
	}

	clauses := make([]models.QueryClause, 0, len(models.AllWorkflowStates()))
	for _, state := range models.AllWorkflowStates() {
		clauses = query.AppendFilterClause(clauses, models.QueryFieldState,
			models.QueryOperatorIs, state.String())
	}
	prospective := query.WithFields(shell.Outliner().FilterQuery,
		[]models.QueryField{models.QueryFieldState}, clauses)
	return prospectiveOutlinerStateCounts(shell.Outliner(), prospective)
}

// prospectiveOutlinerStateCounts runs the canonical row projection on an
// isolated model copy. Expanding task trees and sections keeps presentation
// folds from changing facet totals, while rebuilding through the normal mode
// pipeline preserves all actual scope and visibility rules.
func prospectiveOutlinerStateCounts(
	outlinerView *outliner.OutlinerModel,
	prospective models.Query,
) map[models.WorkflowState]int {
	counts := make(map[models.WorkflowState]int, len(models.AllWorkflowStates()))
	if outlinerView == nil {
		return counts
	}

	projection := *outlinerView
	// The copy takes the prospective narrowing through the one writer, so its
	// carriers are derived from the query the count is asked about rather than
	// the one the Outliner is still showing.
	projection.SetFilterQuery(prospective)
	projection.Rows = nil
	projection.OpenAtTask = nil
	projection.Folds.ResetAll()
	projection.Focus.Restore(
		append([]outliner.FocusEntry(nil), outlinerView.Focus.Stack...),
		append([]outliner.CrossProjectFocusEntry(nil), outlinerView.Focus.CrossStack...),
		outlinerView.Focus.Zoom,
	)
	projection.Population.Invalidate()
	projection.RebuildRows()

	// A matching descendant can retain a non-matching ancestor as hierarchy
	// context. Count only rows that satisfy the prospective predicate themselves.
	// The facet half alone. Under a search projection the isolated copy carries
	// searchProjection != nil, so its rebuild yields fuzzy-scored rows that the
	// full work-mode predicate would then substring-gate with searchRemainder,
	// undercounting; facetPredicateAt is correct in both branches and applies the
	// facets exactly once.
	now := time.Now()
	matches := projection.FacetPredicateAt(now)
	seen := make(map[taskref.TaskIdentity]struct{})
	for _, row := range projection.Rows {
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		project := projection.ResolveProjectForRow(row)
		if !matches(row.Task, project) {
			continue
		}
		projectID := row.ProjectID
		if projectID == "" && project != nil {
			projectID = project.ID
		}
		identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: row.Task.ID}
		if _, duplicate := seen[identity]; duplicate {
			continue
		}
		seen[identity] = struct{}{}
		counts[row.Task.State()]++
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
func (c *Controller) NewFilterTagsPicker(shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	tagList := query.BuildProjectsTagList(c.facetCensusProjects(shell), false)
	source := valuepicker.TaskTagListSource(tagList, false)
	selected := []listsurface.ListItemID(nil)
	if shell.Outliner() != nil {
		for tag := range shell.Outliner().Filter.Tags {
			selected = append(selected, listsurface.ListItemID("tag:"+tag))
		}
	}
	selected = source.RetainKnownIDs(selected)
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderFilterTags,
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
func (c *Controller) FilterScopeProjects(shell Shell) []*models.Project {
	if shell.Outliner() != nil && len(shell.Outliner().Population.Projects) > 0 {
		return shell.Outliner().WorkModeScopeProjects()
	}
	return shell.TaskWorkspaceProjects()
}

// facetCensusProjects is FilterScopeProjects narrowed to the hidden population
// the view admits, so a facet row's count is what choosing it draws: hidden work
// counts only once fH opens it, and then it is all that counts. The narrowing
// copies the project shell alone, judging inheritance against the original tree.
func (c *Controller) facetCensusProjects(shell Shell) []*models.Project {
	projects := c.FilterScopeProjects(shell)
	mode := query.ProjectPopulationExclude
	if shell.Outliner() != nil {
		mode = shell.Outliner().Filter.HiddenMode
	}
	if mode == query.ProjectPopulationInclude {
		return projects
	}
	wantHidden := mode == query.ProjectPopulationOnly
	narrowed := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		census := *project
		census.Tasks = make([]*models.Task, 0, len(project.Tasks))
		for _, task := range project.Tasks {
			if task != nil && models.TaskEffectivelyHidden(project, task) == wantHidden {
				census.Tasks = append(census.Tasks, task)
			}
		}
		narrowed = append(narrowed, &census)
	}
	return narrowed
}

// newFilterContextsPicker is the fc multi-select picker over the active scope's
// context vocabulary, seeded from the active context filter facet. The rows
// compose with OR because a task carries at most one context, and the leading
// taskCountLabel is the census wording both context rows carry.
func taskCountLabel(count int) string {
	return fmt.Sprintf("%d %s", count, listsurface.Pluralize(count, "task", "tasks"))
}

// no-context row names the tasks the vocabulary does not cover.
func (c *Controller) NewFilterContextsPicker(shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	projects := c.facetCensusProjects(shell)
	facets := query.BuildTagFacetList(projects, false)
	items := make([]listsurface.ListItem, 0, len(facets)+1)
	items = append(items, listsurface.ListItem{
		ID:            listsurface.ListItemID("filter-context:" + query.NoContextFilterKey),
		PrimaryText:   "No context",
		SecondaryText: taskCountLabel(query.CountTasksWithoutContext(projects)),
		Aliases:       []string{"no context", "none", "##0"},
		Data:          query.NoContextFilterKey,
	})
	for _, facet := range facets {
		if facet.ContextCount == 0 {
			continue
		}
		items = append(items, listsurface.ListItem{
			ID:            listsurface.ListItemID("filter-context:" + facet.Name),
			PrimaryText:   "##" + facet.Name,
			SecondaryText: taskCountLabel(facet.ContextCount),
			Aliases:       []string{facet.Name, "##" + facet.Name},
			Data:          facet.Name,
		})
	}
	selected := []listsurface.ListItemID(nil)
	if shell.Outliner() != nil {
		filter := &shell.Outliner().Filter.CommonFilterState
		if filter.NoContext {
			selected = append(selected, listsurface.ListItemID("filter-context:"+query.NoContextFilterKey))
		}
		for name := range filter.Contexts {
			selected = append(selected, listsurface.ListItemID("filter-context:"+name))
		}
	}
	source := listsurface.ListSource{Items: items}
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderFilterContexts,
		"Filter by Context",
		"keep matching tasks",
		source,
		true,
		source.RetainKnownIDs(selected),
	)
}

// NewFilterProjectsPicker is the f% multi-select picker over the loaded scope's
// projects, seeded from the active project facet. Each row hands back the slug
// the %project clause spells, and the rows compose with OR because a task
// belongs to exactly one project. A seeded slug no row carries any more — a
// renamed or retired project — is dropped by RetainKnownIDs, so the next apply
// writes it away.
func (c *Controller) NewFilterProjectsPicker(shell Shell,
	request operation.OperationExecutionRequest,
) (*valuepicker.OperationValuePickerModel, error) {
	projects := slices.Clone(c.facetCensusProjects(shell))
	slices.SortStableFunc(projects, func(a, b *models.Project) int {
		return cmp.Or(
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.Slug, b.Slug),
		)
	})
	items := make([]listsurface.ListItem, 0, len(projects))
	seen := make(map[string]bool, len(projects))
	for _, project := range projects {
		if project == nil || project.Slug == "" || seen[project.Slug] {
			continue
		}
		seen[project.Slug] = true
		count := 0
		for _, task := range project.Tasks {
			if task != nil {
				count++
			}
		}
		items = append(items, listsurface.ListItem{
			ID:            listsurface.ListItemID("filter-project:" + project.Slug),
			PrimaryText:   project.Name,
			SecondaryText: "[" + project.Slug + "] · " + taskCountLabel(count),
			Aliases:       []string{project.Slug, "%" + project.Slug},
			Data:          project.Slug,
		})
	}
	selected := []listsurface.ListItemID(nil)
	if shell.Outliner() != nil {
		for _, slug := range shell.Outliner().Filter.Projects {
			selected = append(selected, listsurface.ListItemID("filter-project:"+slug))
		}
	}
	source := listsurface.ListSource{Items: items}
	return valuepicker.NewOperationValuePickerModel(
		request,
		operation.ValueProviderFilterProjects,
		"Filter by Projects",
		"keep tasks in these projects",
		source,
		true,
		source.RetainKnownIDs(selected),
	)
}
