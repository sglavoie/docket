package tui

import (
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// buildOutlinerModeInput assembles the common scoped input all canonical mode
// strategies consume, resolving durable planning records once.
func (m *OutlinerModel) buildOutlinerModeInput(projects []*models.Project) OutlinerModeInput {
	foldOverrides := make(map[TaskIdentity]bool, len(m.expandedNodes))
	for identity, expanded := range m.expandedNodes {
		foldOverrides[identity] = !expanded
	}
	return OutlinerModeInput{
		Projects:        projects,
		DailyPlanIDs:    m.resolveDailyPlanIDs(projects),
		DailyPlanOrder:  m.resolveDailyPlanOrder(projects),
		FocusQueue:      planningFocusQueue(m.runtime),
		Scorer:          m.runtime,
		ChildDepthLimit: -1,
		FoldOverrides:   foldOverrides,
		Now:             time.Now(),
	}
}

// buildCanonicalModeRows is the single row-pipeline entry point for all ten
// canonical modes. Scope, zoom candidates, common filters, search, and fixed
// mode order are assembled once; the catalog strategy changes only membership
// and grouping.
func (m *OutlinerModel) buildCanonicalModeRows() bool {
	mode, ok := OutlinerModeByID(m.primaryMode)
	if !ok {
		return false
	}

	projects := slices.Clone(m.workModeScopeProjects())
	sortProjectsByDisplayOrder(projects)
	input := m.buildOutlinerModeInput(projects)
	input.IncludeSnoozed = m.hasExplicitSnoozedFilter()
	input.CandidateTasks = make(map[string][]*models.Task, len(projects))
	for _, project := range projects {
		if project != nil {
			input.CandidateTasks[project.ID] = flattenTaskPointers(m.primaryModeRoots(project))
		}
	}

	switch mode.RowStrategy {
	case RowStrategyAll, RowStrategyInbox, RowStrategyNext, RowStrategyBacklog:
		m.buildPrimaryModeRows()
	case RowStrategyDue:
		m.appendFilteredModeSections(BuildDueModeSections(input))
	case RowStrategyPriority:
		m.appendFilteredModeSections(BuildPriorityModeSections(input))
	case RowStrategyBlocked:
		m.appendFilteredFlatModeRows(BuildBlockedModeRows(input))
	case RowStrategyParked:
		m.appendFilteredFlatModeRows(BuildParkedModeRows(input))
	case RowStrategyReview:
		m.buildReviewModeRows()
	default:
		return false
	}
	m.pruneInvisibleSelections()
	m.clampCursor()
	return true
}

// appendFilteredModeSections composes the shared per-task filter predicate over
// a mode's fixed grouped order, emitting only non-empty sections.
func (m *OutlinerModel) appendFilteredModeSections(sections []OutlinerModeSection) {
	for _, section := range sections {
		rows := make([]OutlinerModeRow, 0, len(section.Rows))
		for _, row := range section.Rows {
			if row.Task != nil && m.workModeFilterPredicate(row.Project)(row.Task) {
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

func planningFocusQueue(runtime *config.RuntimeState) []config.FocusQueueItem {
	if runtime == nil {
		return nil
	}
	return runtime.Planning().GetFocusQueue()
}

// resolveDailyPlanIDs returns the identity set of daily-planned tasks that
// exist in the scope projects. Derived from the durable plan so it does not
// depend on any externally-maintained indicator cache.
func (m *OutlinerModel) resolveDailyPlanIDs(projects []*models.Project) map[TaskIdentity]bool {
	ids := make(map[TaskIdentity]bool)
	if m.runtime == nil {
		return ids
	}
	for _, item := range m.runtime.Planning().GetDailyPlan() {
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

// resolveDailyPlanOrder returns the daily-plan tasks in their durable order,
// scoped to loaded projects.
func (m *OutlinerModel) resolveDailyPlanOrder(projects []*models.Project) []FocusTask {
	if m.runtime == nil {
		return nil
	}
	plan := m.runtime.Planning().GetDailyPlan()
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

// appendFilteredFlatModeRows appends the flat Next/Backlog mode rows, keeping
// only those that pass the active per-project composite predicate. These
// horizons carry no descendants, so each row is judged independently.
func (m *OutlinerModel) appendFilteredFlatModeRows(modeRows []OutlinerModeRow) {
	for _, mr := range modeRows {
		if mr.Task == nil {
			continue
		}
		if !m.workModeFilterPredicate(mr.Project)(mr.Task) {
			continue
		}
		m.rows = append(m.rows, m.outlinerRowFromModeRow(mr))
	}
}

// workModeTaskFilter adapts workModeFilterPredicate into the project-aware
// TaskFilter the Today row builder consumes, caching each project's predicate so
// the closure is built once per project rather than once per task.
func (m *OutlinerModel) workModeTaskFilter() func(*models.Task, *models.Project) bool {
	cache := make(map[*models.Project]func(*models.Task) bool)
	return func(t *models.Task, p *models.Project) bool {
		predicate, ok := cache[p]
		if !ok {
			predicate = m.workModeFilterPredicate(p)
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
		prefix:             mr.TreePrefix,
		hasVisibleChildren: mr.Depth == 0 && len(mr.Task.Children) > 0,
	}
	if m.project == nil && mr.Project != nil {
		row.ProjectID = mr.Project.ID
	}
	return row
}
