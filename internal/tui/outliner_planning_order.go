package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// planningReorderKind names the durable planning list a J/K move reorders in a
// planning-order context. The Today section reorders the Daily Plan; Next
// reorders the Focus Queue (the Next order index). Neither touches task
// hierarchy.
type planningReorderKind uint8

const (
	reorderNextOrder planningReorderKind = iota
	reorderToday
)

// OutlinerPlanningReorderMsg carries a durable planning-order reorder produced by
// J/K in a dedicated planning mode or an All-mode planning section. The
// Outliner owns the row ordering, so it computes the full new workspace order;
// the shell persists it through the planning transaction path and reloads the
// cross outliner with the cursor kept on the moved task. Because the whole
// reorder is a single planning commit it survives a restart and never mutates
// the task tree.
type OutlinerPlanningReorderMsg struct {
	Kind  planningReorderKind
	Order []config.ProjectTaskLocator
}

// isPlanningReorderMode reports whether J/K reorders a durable planning list
// rather than task hierarchy. Dedicated Today/Next modes and their synthetic
// All-mode groups share the same reorder path.
func (m *OutlinerModel) isPlanningReorderMode() bool {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]
	if row.IsPinnedSection {
		return false
	}
	if (m.primaryMode == ModeToday || m.primaryMode == ModeNext) && row.isTaskRow() {
		return true
	}
	return row.IsTodaySection || row.IsNextSection
}

// planningReorderMove builds the durable planning-order reorder for a J/K move
// in a planning projection. The target is the adjacent visible reorderable row,
// and the two visible identities are swapped in place within the complete
// workspace-wide order. Hidden planning members stay put relative to the pair.
func (m *OutlinerModel) planningReorderMove(delta int) (tea.Model, tea.Cmd) {
	if !m.isPlanningReorderMode() {
		return m, nil
	}
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}

	kind := reorderToday
	order := m.globalTodayOrder()
	row := m.rows[m.cursor]
	if m.primaryMode == ModeNext || row.IsNextSection {
		kind = reorderNextOrder
		order = m.globalNextOrder()
	}

	moved := config.ProjectTaskLocator{ProjectID: project.ID, TaskID: task.ID}
	movedPos := locatorIndex(order, moved)
	if movedPos < 0 {
		// The cursor task is no longer a member of the planning list.
		return m, nil
	}

	// Resolve the target from the adjacent *visible* reorderable planning row, not
	// from the neighbor in the unfiltered durable order — otherwise a scope or
	// filter could swap the cursor with an invisible task.
	target, _, ok := m.adjacentVisiblePlanningMember(order, delta)
	if !ok {
		return m, nil // visible boundary: nothing to swap with.
	}
	targetPos := locatorIndex(order, target)
	if targetPos < 0 {
		return m, nil
	}
	order[movedPos], order[targetPos] = order[targetPos], order[movedPos]

	msg := OutlinerPlanningReorderMsg{
		Kind:  kind,
		Order: order,
	}
	return m, func() tea.Msg { return msg }
}

// adjacentVisiblePlanningMember returns the locator and task of the nearest
// visible row — scanning m.rows from the cursor in the direction of delta — whose
// task is a durable planning member (present in order). Separators and top-level
// section headers bound the active planning section; planning project headers
// are transparent because they do not partition the durable order. It reports
// false at a visible boundary when no such row exists.
func (m *OutlinerModel) adjacentVisiblePlanningMember(order []config.ProjectTaskLocator, delta int) (config.ProjectTaskLocator, *models.Task, bool) {
	members := make(map[config.ProjectTaskLocator]bool, len(order))
	for _, loc := range order {
		members[loc] = true
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	for i := m.cursor + step; i >= 0 && i < len(m.rows); i += step {
		row := m.rows[i]
		if row.IsSectionHeader || row.IsSeparator || row.IsPinnedSection {
			break
		}
		// Planning project headers provide context without partitioning the
		// workspace-wide order. Skip them so J/K can still swap adjacent visible
		// Today/Next members across a project boundary. Other project headers
		// continue to delimit hierarchy reorder buckets.
		if row.IsProjectHeader {
			if row.IsTodaySection || row.IsNextSection {
				continue
			}
			break
		}
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		identity := m.taskIdentityForRow(row)
		loc := config.ProjectTaskLocator{ProjectID: identity.ProjectID, TaskID: identity.TaskID}
		if members[loc] {
			return loc, row.task, true
		}
	}
	return config.ProjectTaskLocator{}, nil, false
}

// globalNextOrder returns the workspace-wide Next sequence as project-qualified
// locators, ordered exactly as BuildNextModeRows renders it. Reordering over
// this full sequence (rather than only the visible scope) keeps the global order
// coherent.
func (m *OutlinerModel) globalNextOrder() []config.ProjectTaskLocator {
	projects := m.projectsInCatalogOrder(m.planningProjects())
	input := m.buildOutlinerModeInput(projects)
	input.CandidateTasks = nil
	input.TaskFilter = nil
	input.TaskExclude = nil
	input.IncludeSnoozed = true
	rows := BuildNextModeRows(input)
	order := make([]config.ProjectTaskLocator, 0, len(rows))
	for _, r := range rows {
		if r.Task == nil || r.Project == nil {
			continue
		}
		order = append(order, config.ProjectTaskLocator{ProjectID: r.Project.ID, TaskID: r.Task.ID})
	}
	return order
}

// globalTodayOrder returns the workspace-wide Today list in the same durable
// project-qualified order used by the display.
func (m *OutlinerModel) globalTodayOrder() []config.ProjectTaskLocator {
	plan := m.resolveTodayOrder(m.projectsInCatalogOrder(m.planningProjects()))
	order := make([]config.ProjectTaskLocator, 0, len(plan))
	for _, ft := range plan {
		if ft.Task == nil || ft.Project == nil {
			continue
		}
		order = append(order, config.ProjectTaskLocator{ProjectID: ft.Project.ID, TaskID: ft.Task.ID})
	}
	return order
}

func (m *OutlinerModel) planningProjects() []*models.Project {
	if m == nil {
		return nil
	}
	if m.project != nil {
		return []*models.Project{m.project}
	}
	return m.loadedProjects
}

// locatorIndex returns the position of loc in order, or -1 when absent.
func locatorIndex(order []config.ProjectTaskLocator, loc config.ProjectTaskLocator) int {
	for i, item := range order {
		if item == loc {
			return i
		}
	}
	return -1
}

// handleOutlinerPlanningReorder persists a global Outliner planning-order reorder
// through the checked planning commit, rebuilds the in-memory planning caches, and
// reloads the cross outliner so the new order renders with the cursor kept on the
// moved task. The reorder is planning-domain only: task files are never rewritten.
func (m *AppModel) handleOutlinerPlanningReorder(msg OutlinerPlanningReorderMsg) tea.Cmd {
	order := make([]app.TaskLocator, 0, len(msg.Order))
	for _, loc := range msg.Order {
		order = append(order, app.TaskLocator{ProjectID: loc.ProjectID, TaskID: loc.TaskID})
	}
	var err error
	switch msg.Kind {
	case reorderNextOrder:
		err = m.appService.ReorderNext(order)
	case reorderToday:
		err = m.appService.ReorderToday(order)
	}
	if err != nil {
		m.err = err
		return errorCmd("%s", err)
	}

	m.refreshRuntime()
	m.rebuildPlanningFromRuntime()
	if m.crossOutliner != nil {
		m.crossOutliner.crossForceRebuild = true
	}
	m.setInfoStatus("Reordered planning list")
	return tea.Batch(m.refreshTaskViewsAfterPlanningMutation()...)
}
