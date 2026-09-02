package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
)

// handleOutlinerPlanningReorder persists a global Outliner planning-order reorder
// through the checked planning commit, rebuilds the in-memory planning caches, and
// reloads the cross outliner so the new order renders with the cursor kept on the
// moved task. The reorder is planning-domain only: task files are never rewritten.
//
// The Outliner computes the whole new workspace order and raises
// OutlinerPlanningReorderMsg; only the shell holds the application service that
// can commit it. See outliner_planning_order.go for the row side.
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
