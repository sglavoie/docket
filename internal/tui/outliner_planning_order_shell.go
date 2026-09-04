package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/status"
)

// handleOutlinerPlanningReorder persists a global Outliner planning-order reorder
// through the checked planning commit, rebuilds the in-memory planning caches, and
// reloads the cross outliner so the new order renders with the cursor kept on the
// moved task. The reorder is planning-domain only: task files are never rewritten.
//
// The Outliner computes the whole new workspace order and raises
// OutlinerPlanningReorderMsg; only the shell holds the application service that
// can commit it. See outliner_planning_order.go for the row side.
func (m *AppModel) handleOutlinerPlanningReorder(msg outliner.OutlinerPlanningReorderMsg) tea.Cmd {
	order := make([]app.TaskLocator, 0, len(msg.Order))
	for _, loc := range msg.Order {
		order = append(order, app.TaskLocator{ProjectID: loc.ProjectID, TaskID: loc.TaskID})
	}
	var err error
	switch msg.Kind {
	case outliner.ReorderNextOrder:
		err = m.domain.service.ReorderNext(order)
	case outliner.ReorderToday:
		err = m.domain.service.ReorderToday(order)
	}
	if err != nil {
		m.feedback.err = err
		return status.ErrorCmd("%s", err)
	}

	m.refreshRuntime()
	m.rebuildPlanningFromRuntime()
	if m.crossOutliner != nil {
		m.crossOutliner.Population.Invalidate()
	}
	m.setInfoStatus("Reordered planning list")
	return tea.Batch(m.refreshTaskViewsAfterPlanningMutation()...)
}
