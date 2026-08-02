package tui

import (
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// workspaceFilterResolver is a temporary compile-only placeholder for the
// dependency-filter plumbing removed in session 18.
type workspaceFilterResolver struct{}

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
	// One resolver over one loaded workspace snapshot is shared by every mode
	// section in this projection. In particular, a completed cross-project
	// dependency must not leave its dependent visible under fb.
	resolver := m.newWorkspaceFilterResolver(projects)
	input.TaskFilter = m.workModeTaskFilterWithResolverAt(now, resolver)
	return input
}

func (m *OutlinerModel) newWorkspaceFilterResolver(
	fallback []*models.Project,
) *workspaceFilterResolver {
	if m == nil {
		return nil
	}
	if m.workspaceResolver != nil {
		return m.workspaceResolver
	}
	projects := m.loadedProjects
	if len(projects) == 0 {
		if m.project != nil {
			projects = []*models.Project{m.project}
		} else {
			projects = fallback
		}
	}
	if len(projects) == 0 {
		return nil
	}
	return &workspaceFilterResolver{}
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
func (m *OutlinerModel) buildCanonicalModeRows() bool {
	mode, ok := OutlinerModeByID(m.primaryMode)
	if !ok {
		return false
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

	switch mode.RowStrategy {
	case RowStrategyAll, RowStrategyInbox, RowStrategyToday, RowStrategyNext, RowStrategyBacklog:
		m.buildPrimaryModeRowsWithInput(projects, input, pinned.projected)
	case RowStrategyDue:
		m.appendFilteredModeSections(BuildDueModeSections(input), pinned.projected, input.TaskFilter)
	default:
		return false
	}
	m.pruneInvisibleSelections()
	m.clampCursor()
	return true
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
		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader:  true,
			IsComputedHeader: true,
			SectionName:      section.Name,
			SectionCount:     len(rows),
			CollapseKey:      section.CollapseKey,
		})
		if m.collapsedSections[section.CollapseKey] {
			continue
		}
		for _, row := range rows {
			m.rows = append(m.rows, m.outlinerRowFromModeRow(row))
		}
	}
}

func planningNextOrder(runtime *config.RuntimeState) []config.ProjectTaskLocator {
	if runtime == nil {
		return nil
	}
	return runtime.Planning().GetNextOrder()
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
func (m *OutlinerModel) resolveTodayOrder(projects []*models.Project) []FocusTask {
	if m.runtime == nil {
		return nil
	}
	plan := m.runtime.Planning().GetToday()
	order := make([]FocusTask, 0, len(plan))
	for _, item := range plan {
		for _, p := range projects {
			if p == nil || p.ID != item.ProjectID {
				continue
			}
			if t := p.GetTaskByID(item.TaskID); t != nil {
				order = append(order, FocusTask{Task: t, Project: p})
			}
			break
		}
	}
	return order
}

func (m *OutlinerModel) workModeTaskFilterAt(now time.Time) func(*models.Task, *models.Project) bool {
	return m.workModeTaskFilterWithResolverAt(now, m.newWorkspaceFilterResolver(nil))
}

func (m *OutlinerModel) workModeTaskFilterWithResolverAt(
	now time.Time,
	resolver *workspaceFilterResolver,
) func(*models.Task, *models.Project) bool {
	cache := make(map[*models.Project]func(*models.Task) bool)
	return func(t *models.Task, p *models.Project) bool {
		predicate, ok := cache[p]
		if !ok {
			predicate = m.workModeFilterPredicateWithResolverAt(p, resolver, now)
			cache[p] = predicate
		}
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
