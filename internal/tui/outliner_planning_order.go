package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// planningReorderKind names the durable planning list a J/K move reorders in a
// planning-order context. The Today section reorders the Daily Plan; Next
// reorders the Focus Queue (the Next order index). Neither touches task
// hierarchy.
type planningReorderKind uint8

const (
	reorderFocusQueue planningReorderKind = iota
	reorderDailyPlan
)

// OutlinerPlanningReorderMsg carries a durable planning-order reorder produced by
// the global Outliner's J/K movement in the Today section or Next mode. The
// Outliner owns the row ordering, so it computes the full new workspace order;
// the shell persists it through the planning transaction path and reloads the
// cross outliner with the cursor kept on the moved task. Because the whole
// reorder is a single planning commit it survives a restart and never mutates
// the task tree.
type OutlinerPlanningReorderMsg struct {
	Kind         planningReorderKind
	Order        []config.ProjectTaskLocator
	MovedTaskID  string
	MovedProject string
}

// isPlanningReorderMode reports whether J/K reorders a durable planning list
// rather than task hierarchy: an explicit member under the global Outliner's
// synthetic Today section reorders the Daily Plan, while Next reorders the Focus
// Queue. Context descendants beneath Today also return true so J/K is a quiet
// no-op instead of accidentally mutating the underlying hierarchy.
func (m *OutlinerModel) isPlanningReorderMode() bool {
	if m.project != nil {
		return false
	}
	if m.workNextMode {
		return true
	}
	return m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].IsTodaySection
}

// planningReorderMove builds the durable planning-order reorder for a J/K move in
// the synthetic Today section or Next horizon. The target is the adjacent visible reorderable
// planning row (in the move direction), and the two visible identities are swapped
// in place within the complete workspace-wide planning order — so a scope or filter
// can never swap the cursor with a hidden neighbor and hidden planning members stay
// put relative to the requested pair. It returns a persist command, or a silent
// no-op at a visible boundary or when the cursor is not a reorderable planning
// member.
func (m *OutlinerModel) planningReorderMove(delta int) (tea.Model, tea.Cmd) {
	if !m.isPlanningReorderMode() {
		return m, nil
	}
	task, project := m.getTaskRef()
	if task == nil || project == nil {
		return m, nil
	}

	kind := reorderDailyPlan
	order := m.globalDailyPlanOrder()
	if m.workNextMode {
		kind = reorderFocusQueue
		order = m.globalNextOrder()
	}

	moved := config.ProjectTaskLocator{ProjectID: project.ID, TaskID: task.ID}
	movedPos := locatorIndex(order, moved)
	if movedPos < 0 {
		// The cursor task is not a member of the planning list (e.g. a Today
		// descendant expanded under a planned root); it cannot be reordered.
		return m, nil
	}

	// Resolve the target from the adjacent *visible* reorderable planning row, not
	// from the neighbor in the unfiltered durable order — otherwise a scope or
	// filter could swap the cursor with an invisible task.
	target, targetTask, ok := m.adjacentVisiblePlanningMember(order, delta)
	if !ok {
		return m, nil // visible boundary: nothing to swap with.
	}
	if kind == reorderDailyPlan && TodayStateOrder(task.State()) != TodayStateOrder(targetTask.State()) {
		// Today renders state buckets before durable Daily Plan position. Swapping
		// across buckets would change persisted order without moving either task
		// visibly, so treat the bucket edge as a silent boundary.
		return m, nil
	}
	targetPos := locatorIndex(order, target)
	if targetPos < 0 {
		return m, nil
	}
	order[movedPos], order[targetPos] = order[targetPos], order[movedPos]

	msg := OutlinerPlanningReorderMsg{
		Kind:         kind,
		Order:        order,
		MovedTaskID:  task.ID,
		MovedProject: project.ID,
	}
	return m, func() tea.Msg { return msg }
}

// adjacentVisiblePlanningMember returns the locator and task of the nearest
// visible row — scanning m.rows from the cursor in the direction of delta — whose
// task is a durable planning member (present in order). Separators, headers, and
// non-reorderable Today descendants (visible task rows that are not plan members)
// are skipped so J/K targets the true visible planning neighbor. It reports false
// at a visible boundary when no such row exists.
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
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		loc := config.ProjectTaskLocator{ProjectID: row.ProjectID, TaskID: row.task.ID}
		if members[loc] {
			return loc, row.task, true
		}
	}
	return config.ProjectTaskLocator{}, nil, false
}

// globalNextOrder returns the workspace-wide Next sequence as project-qualified
// locators, ordered exactly as BuildNextModeRows renders it: focus-queue order
// with unindexed StateNext members stable at the end. Reordering over this full
// sequence (rather than only the visible scope) keeps the global order coherent.
func (m *OutlinerModel) globalNextOrder() []config.ProjectTaskLocator {
	rows := BuildNextModeRows(m.buildOutlinerModeInput(m.loadedProjects))
	order := make([]config.ProjectTaskLocator, 0, len(rows))
	for _, r := range rows {
		if r.Task == nil || r.Project == nil {
			continue
		}
		order = append(order, config.ProjectTaskLocator{ProjectID: r.Project.ID, TaskID: r.Task.ID})
	}
	return order
}

// globalDailyPlanOrder returns the workspace-wide Daily Plan as project-qualified
// locators in their durable plan order (not the state-sorted Today display order),
// so J/K moves a task within the persisted plan.
func (m *OutlinerModel) globalDailyPlanOrder() []config.ProjectTaskLocator {
	plan := m.resolveDailyPlanOrder(m.loadedProjects)
	order := make([]config.ProjectTaskLocator, 0, len(plan))
	for _, ft := range plan {
		if ft.Task == nil || ft.Project == nil {
			continue
		}
		order = append(order, config.ProjectTaskLocator{ProjectID: ft.Project.ID, TaskID: ft.Task.ID})
	}
	return order
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
	var err error
	switch msg.Kind {
	case reorderFocusQueue:
		items := make([]config.FocusQueueItem, 0, len(msg.Order))
		for _, loc := range msg.Order {
			items = append(items, config.FocusQueueItem{TaskID: loc.TaskID, ProjectID: loc.ProjectID})
		}
		err = m.commitPlanningMutation(func(p *config.PlanningState) { p.SetFocusQueue(items) })
	case reorderDailyPlan:
		items := m.dailyPlanItemsForOrder(msg.Order)
		err = m.commitPlanningMutation(func(p *config.PlanningState) { p.SetDailyPlan(items) })
	}
	if err != nil {
		return errorCmd("%s", err)
	}

	m.rebuildPlanningFromRuntime()
	if m.crossOutliner != nil {
		m.crossOutliner.crossForceRebuild = true
	}
	m.setInfoStatus("Reordered planning list")
	return tea.Batch(m.refreshTaskViewsAfterSave()...)
}

// dailyPlanItemsForOrder rebuilds Daily Plan items in the given order, preserving
// each task's existing planned date and defaulting new entries to today so a
// reorder never rewrites the scheduled day.
func (m *AppModel) dailyPlanItemsForOrder(order []config.ProjectTaskLocator) []config.DailyPlanItem {
	today := time.Now().Format("2006-01-02")
	dates := make(map[config.ProjectTaskLocator]string)
	for _, item := range m.runtime.Planning().GetDailyPlan() {
		dates[config.ProjectTaskLocator{ProjectID: item.ProjectID, TaskID: item.TaskID}] = item.PlannedDate
	}
	items := make([]config.DailyPlanItem, 0, len(order))
	for _, loc := range order {
		date := dates[loc]
		if date == "" {
			date = today
		}
		items = append(items, config.DailyPlanItem{TaskID: loc.TaskID, ProjectID: loc.ProjectID, PlannedDate: date})
	}
	return items
}
