package tui

import (
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// buildOutlinerModeInput assembles the common scoped input all canonical mode
// strategies consume, resolving durable planning records once.
func (m *OutlinerModel) buildOutlinerModeInput(projects []*models.Project) OutlinerModeInput {
	now := time.Now()
	input := OutlinerModeInput{
		Projects:   projects,
		TodayIDs:   m.resolveTodayIDs(projects),
		TodayOrder: m.resolveTodayOrder(projects),
		NextOrder:  planningNextOrder(m.runtime),
		Now:        now,
	}
	input.TaskFilter = m.workModeTaskFilterAt(now)
	return input
}

// projectsInCatalogOrder returns the supplied project subset in the workspace
// catalog's durable order. Unknown projects retain their input order after all
// cataloged projects so incomplete test/runtime snapshots remain deterministic.
func (m *OutlinerModel) projectsInCatalogOrder(projects []*models.Project) []*models.Project {
	ordered := slices.Clone(projects)
	if m == nil || m.runtime == nil {
		return ordered
	}
	ranks := make(map[string]int, len(m.runtime.Catalog().Projects))
	for index, project := range m.runtime.Catalog().Projects {
		ranks[project.ID] = index
	}
	const unranked = int(^uint(0) >> 1)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := unranked, unranked
		if ordered[i] != nil {
			if rank, ok := ranks[ordered[i].ID]; ok {
				left = rank
			}
		}
		if ordered[j] != nil {
			if rank, ok := ranks[ordered[j].ID]; ok {
				right = rank
			}
		}
		return left < right
	})
	return ordered
}

// buildCanonicalModeRows is the single row-pipeline entry point for all six
// canonical modes. Scope, zoom candidates, common filters, search, and fixed
// mode order are assembled once; the catalog strategy changes only membership
// and grouping.
func (m *OutlinerModel) buildCanonicalModeRows() {
	if m.hasSearchProjection() {
		m.buildSearchProjectionRows()
		m.pruneInvisibleSelections()
		m.clampCursor()
		return
	}

	mode, ok := OutlinerModeByID(m.primaryMode)
	if !ok {
		return
	}

	projects := m.projectsInCatalogOrder(m.workModeScopeProjects())
	input := m.buildOutlinerModeInput(projects)
	input.IncludeSnoozed = m.hasExplicitSnoozedFilter()
	input.IncludeTerminal = m.hasExplicitTerminalFilter()
	input.CandidateTasks = make(map[string][]*models.Task, len(projects))
	for _, project := range projects {
		if project != nil {
			input.CandidateTasks[project.ID] = flattenTaskPointers(m.primaryModeRoots(project))
		}
	}
	pinned := m.buildPinnedSection(input)
	m.appendPinnedSection(pinned)
	pinnedBoundary := len(m.rows)

	switch mode.RowStrategy {
	case RowStrategyAll, RowStrategyInbox, RowStrategyToday, RowStrategyNext, RowStrategyBacklog:
		m.buildPrimaryModeRowsWithInput(projects, input, pinned.projected)
	case RowStrategyDue:
		m.appendFilteredModeSections(BuildDueModeSections(input), pinned.projected, input.TaskFilter)
	default:
		return
	}
	m.separatePinnedSection(pinnedBoundary)
	m.pruneInvisibleSelections()
	m.clampCursor()
}

// separatePinnedSection closes Pinned with a rule so the promoted rows never
// read as the first group of the mode below them.
func (m *OutlinerModel) separatePinnedSection(boundary int) {
	if boundary == 0 || boundary >= len(m.rows) {
		return
	}
	m.rows = slices.Insert(m.rows, boundary, outlinerRow{IsSeparator: true})
}

// appendFilteredModeSections composes the shared per-task filter predicate over
// a mode's fixed grouped order, emitting only non-empty sections.
func (m *OutlinerModel) appendFilteredModeSections(
	sections []OutlinerModeSection,
	excluded map[TaskIdentity]bool,
	taskFilter func(*models.Task, *models.Project) bool,
) {
	for _, section := range sections {
		rows := make([]OutlinerModeRow, 0, len(section.Rows))
		for _, row := range section.Rows {
			if row.Task != nil &&
				!excluded[taskIdentityFor(row.Task, row.Project)] &&
				(taskFilter == nil || taskFilter(row.Task, row.Project)) {
				rows = append(rows, row)
			}
		}
		if len(rows) == 0 {
			continue
		}
		projectedRows := make([]outlinerRow, 0, len(rows))
		for _, row := range rows {
			projectedRows = append(projectedRows, m.outlinerRowFromModeRow(row))
		}
		forest := m.buildSectionRowForest(projectedRows)
		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader:  true,
			IsComputedHeader: true,
			SectionName:      section.Name,
			SectionCount:     forest.count,
			CollapseKey:      section.CollapseKey,
		})
		if m.collapsedSections[section.CollapseKey] {
			continue
		}
		m.flattenSectionRowForest(forest, &m.rows)
	}
}

func planningNextOrder(runtime *config.RuntimeState) []config.ProjectTaskLocator {
	if runtime == nil {
		return nil
	}
	return runtime.Planning().GetNext()
}

// resolveTodayIDs returns the identity set of daily-planned tasks that
// exist in the scope projects. Derived from the durable plan so it does not
// depend on any externally-maintained indicator cache.
func (m *OutlinerModel) resolveTodayIDs(projects []*models.Project) map[TaskIdentity]bool {
	ids := make(map[TaskIdentity]bool)
	if m.runtime == nil {
		return ids
	}
	for _, item := range m.runtime.Planning().GetToday() {
		for _, p := range projects {
			if p == nil || p.ID != item.ProjectID {
				continue
			}
			if t := p.GetTaskByID(item.TaskID); t != nil {
				ids[taskIdentityFor(t, p)] = true
			}
			break
		}
	}
	return ids
}

// resolveTodayOrder returns the daily-plan tasks in their durable order,
// scoped to loaded projects.
func (m *OutlinerModel) resolveTodayOrder(projects []*models.Project) []PlanningTask {
	if m.runtime == nil {
		return nil
	}
	plan := m.runtime.Planning().GetToday()
	order := make([]PlanningTask, 0, len(plan))
	for _, item := range plan {
		for _, p := range projects {
			if p == nil || p.ID != item.ProjectID {
				continue
			}
			if t := p.GetTaskByID(item.TaskID); t != nil {
				order = append(order, PlanningTask{Task: t, Project: p})
			}
			break
		}
	}
	return order
}

func (m *OutlinerModel) workModeTaskFilterAt(now time.Time) func(*models.Task, *models.Project) bool {
	predicate := m.workModeFilterPredicateAt(now)
	return func(t *models.Task, _ *models.Project) bool {
		return predicate(t)
	}
}

// outlinerRowFromModeRow adapts a representation-neutral OutlinerModeRow to a
// concrete flat display row, carrying the owning project ID in cross-project
// scope so badges resolve.
func (m *OutlinerModel) outlinerRowFromModeRow(mr OutlinerModeRow) outlinerRow {
	row := outlinerRow{
		task:               mr.Task,
		Breadcrumb:         mr.Breadcrumb,
		hasVisibleChildren: false,
	}
	if m.project == nil && mr.Project != nil {
		row.ProjectID = mr.Project.ID
	}
	return row
}
