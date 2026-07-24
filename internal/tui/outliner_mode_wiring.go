package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// buildOutlinerModeInput assembles the cross-project input the Today section
// and Backlog row strategy consume, resolving the durable daily-plan and
// focus-queue records against the given scope projects.
func (m *OutlinerModel) buildOutlinerModeInput(projects []*models.Project) OutlinerModeInput {
	return OutlinerModeInput{
		Projects:        projects,
		DailyPlanIDs:    m.resolveDailyPlanIDs(projects),
		DailyPlanOrder:  m.resolveDailyPlanOrder(projects),
		FocusQueue:      m.runtime.Planning().GetFocusQueue(),
		Scorer:          m.runtime,
		ChildDepthLimit: -1,
		Now:             time.Now(),
	}
}

// resolveDailyPlanIDs returns the identity set of daily-planned tasks that
// exist in the scope projects. Derived from the durable plan so it does not
// depend on any externally-maintained indicator cache.
func (m *OutlinerModel) resolveDailyPlanIDs(projects []*models.Project) map[TaskIdentity]bool {
	ids := make(map[TaskIdentity]bool)
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

// buildWorkModeRows populates m.rows for the active computed work horizon
// (Next or Backlog). Rows are flat, fixed-order lists produced by the
// shared mode-row strategies, then narrowed by the same visible filters the tree
// modes apply.
func (m *OutlinerModel) buildWorkModeRows() {
	projects := m.workModeScopeProjects()
	input := m.buildOutlinerModeInput(projects)

	if m.workNextMode {
		m.appendFilteredFlatModeRows(BuildNextModeRows(input))
		return
	}
	m.appendFilteredFlatModeRows(BuildBacklogModeRows(input))
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
