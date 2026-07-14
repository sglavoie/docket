package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

const (
	allTasksMultiSelectEnterMsg = "Multi-select: space=toggle, a=all, A=deselect, ,s/d/b=status"
	allTasksMultiSelectNoSel    = "Multi-select: space=toggle, a=all, A=deselect"
	allTasksMultiSelectWithSel  = "%d selected (space=toggle, ,s/d/b=status, D=delete)"
)

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *AllTasksViewModel) toggleMultiSelectMode() {
	m.ToggleMode(&m.statusMessage, allTasksMultiSelectEnterMsg)
}

// toggleTaskSelection toggles selection of the current task.
func (m *AllTasksViewModel) toggleTaskSelection() {
	item := m.getTaskAtCursor()
	if item == nil {
		return
	}
	m.ToggleAndUpdateStatus(item.Task.ID, &m.statusMessage, allTasksMultiSelectNoSel, allTasksMultiSelectWithSel)
}

// selectAllVisible selects all visible tasks in non-collapsed groups.
func (m *AllTasksViewModel) selectAllVisible() {
	for _, group := range m.groups {
		if m.collapsedGroups[group.Name] {
			continue
		}
		for _, item := range group.Tasks {
			m.selectedTasks[item.Task.ID] = true
		}
	}
	m.UpdateStatus(&m.statusMessage, allTasksMultiSelectNoSel, allTasksMultiSelectWithSel)
}

// deselectAll clears all task selections.
func (m *AllTasksViewModel) deselectAll() {
	m.ClearSelection()
	m.UpdateStatus(&m.statusMessage, allTasksMultiSelectNoSel, allTasksMultiSelectWithSel)
}

// getSelectedBulkItems returns all selected tasks as a flat slice of bulkTaskItem.
func (m *AllTasksViewModel) getSelectedBulkItems() []bulkTaskItem {
	var items []bulkTaskItem
	for _, group := range m.groups {
		for _, item := range group.Tasks {
			if m.selectedTasks[item.Task.ID] {
				items = append(items, bulkTaskItem{Task: item.Task, Project: item.Project})
			}
		}
	}
	return items
}

// bulkTaskIDs returns the selected task IDs for a bulk mutation, or nil (with no
// change) when nothing is selected. All Tasks routes every multiselect mutation
// through the application-layer BulkCoordinator: the batch commits asynchronously
// as one transaction, the shell reloads the touched projects and clears the
// selection on success, and per-view success wording is supplied via
// finalizeBatchResult so the toast reads exactly as it did on the synchronous
// path.
func (m *AllTasksViewModel) bulkTaskIDs() []string {
	return taskIDsFromTaskRefs(m.getSelectedBulkItems())
}

// handleBulkStateChange transitions all selected tasks to the given canonical
// WorkflowState, running the workflow-rule cascade and dropping tasks that
// cannot transition (best-effort SkipInvalid), matching the legacy per-task
// guard.
func (m *AllTasksViewModel) handleBulkStateChange(newState models.WorkflowState) (tea.Model, tea.Cmd) {
	taskIDs := m.bulkTaskIDs()
	if len(taskIDs) == 0 {
		return m, nil
	}
	cmd := m.bulk.TransitionWithWorkflow(
		app.BulkTransitionRequest{
			TaskIDs:     taskIDs,
			Target:      newState,
			Rules:       m.enabledWorkflowRules(),
			UndoAction:  StateVerb(newState) + " Task",
			UndoContext: fmt.Sprintf("%d tasks", len(taskIDs)),
			SkipInvalid: true,
		},
		func(changed int) string {
			if changed == 0 {
				return "No tasks were changed"
			}
			return fmt.Sprintf("Changed %d tasks to %s", changed, newState)
		},
	)
	return m, finalizeBatchResult(cmd, taskIDs, "")
}

// handleBulkPriorityChange sets the priority on all selected tasks across projects.
func (m *AllTasksViewModel) handleBulkPriorityChange(priority int) (tea.Model, tea.Cmd) {
	taskIDs := m.bulkTaskIDs()
	if len(taskIDs) == 0 {
		return m, nil
	}
	cmd := m.bulk.SetPriority(taskIDs, priority)
	summary := fmt.Sprintf("Set priority %d on %d tasks", priority, len(taskIDs))
	return m, finalizeBatchResult(cmd, taskIDs, summary)
}

// handleBulkDueDateChange sets a due date on all selected tasks.
func (m *AllTasksViewModel) handleBulkDueDateChange(daysFromNow int) (tea.Model, tea.Cmd) {
	taskIDs := m.bulkTaskIDs()
	if len(taskIDs) == 0 {
		return m, nil
	}
	dueDate := util.StartOfDay(time.Now().AddDate(0, 0, daysFromNow))
	cmd := m.bulk.SetDueDate(taskIDs, dueDate, false)
	summary := fmt.Sprintf("Due date %s on %d tasks", dueDateLabel(daysFromNow), len(taskIDs))
	return m, finalizeBatchResult(cmd, taskIDs, summary)
}

// handleBulkClearDueDate clears due dates on all selected tasks.
func (m *AllTasksViewModel) handleBulkClearDueDate() (tea.Model, tea.Cmd) {
	taskIDs := m.bulkTaskIDs()
	if len(taskIDs) == 0 {
		return m, nil
	}
	cmd := m.bulk.ClearDueDate(taskIDs)
	summary := fmt.Sprintf("Cleared due date on %d tasks", len(taskIDs))
	return m, finalizeBatchResult(cmd, taskIDs, summary)
}

// handleBulkSectionMove moves all selected tasks to the workflow state their
// target GTD section maps onto. The locked design retires standalone sections, so
// a "send to section X" bulk action is a state transition (see the mapping in
// stateForSectionName), routed through the same workflow-aware batch path.
func (m *AllTasksViewModel) handleBulkSectionMove(targetSection string) (tea.Model, tea.Cmd) {
	target, ok := stateForSectionName(targetSection)
	if !ok {
		m.setStatus(fmt.Sprintf("Unknown section %q", targetSection))
		return m, nil
	}
	taskIDs := m.bulkTaskIDs()
	if len(taskIDs) == 0 {
		return m, nil
	}
	cmd := m.bulk.TransitionWithWorkflow(
		app.BulkTransitionRequest{
			TaskIDs:     taskIDs,
			Target:      target,
			Rules:       m.enabledWorkflowRules(),
			UndoAction:  "Move to Section",
			UndoContext: fmt.Sprintf("%d tasks", len(taskIDs)),
			SkipInvalid: true,
		},
		func(changed int) string {
			if changed == 0 {
				return "No tasks were moved"
			}
			return fmt.Sprintf("Moved %d tasks to %s", changed, targetSection)
		},
	)
	return m, finalizeBatchResult(cmd, taskIDs, "")
}

// handleBulkDelete deletes all selected subtrees through the application layer.
// The durable planning refs (Focus Queue, Daily Plan, dismissals) of every
// removed task ride the delete transaction as a config artifact, mirroring the
// CLI delete; machine-local recent visits are repaired separately by the shell
// after the commit.
func (m *AllTasksViewModel) handleBulkDelete() (tea.Model, tea.Cmd) {
	taskIDs := m.bulkTaskIDs()
	if len(taskIDs) == 0 {
		return m, nil
	}

	// Expand the selected roots to their whole subtrees so the durable planning
	// cleanup keys on every task the delete will remove, not just the roots.
	idsToDelete := make(map[string]bool)
	for _, group := range groupTaskProjectRefsByProject(m.getSelectedBulkItems()) {
		if group.Project == nil {
			continue
		}
		rootIDs := make(map[string]bool, len(group.Items))
		for _, item := range group.Items {
			rootIDs[item.Task.ID] = true
		}
		for id := range collectDescendantIDs(group.Project.Tasks, rootIDs) {
			idsToDelete[id] = true
		}
	}

	// Fold the durable planning-ref cleanup into the delete transaction as the
	// revision-checked planning-domain artifact.
	var artifacts []storage.ArtifactChange
	if m.runtime != nil && m.configManager != nil {
		planning := clonePlanningState(m.runtime.Planning())
		if planning.RemoveDurableTaskRefs(idsToDelete) > 0 {
			arts, err := planningDomainArtifact(m.configManager, planning)
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
				return m, nil
			}
			artifacts = arts
		}
	}

	cmd := m.bulk.Delete(taskIDs, artifacts...)
	return m, finalizeBatchResult(cmd, taskIDs, "")
}

// stateForSectionName maps a legacy GTD section name (as offered by the "send to
// section" bulk action) to the workflow state that realizes it. It is the inverse
// of models.SectionForState; the empty section is the active bucket, which maps to
// the ready state.
func stateForSectionName(section string) (models.WorkflowState, bool) {
	switch section {
	case "":
		return models.StateReady, true
	case models.SectionNext:
		return models.StateNext, true
	case models.SectionInbox:
		return models.StateInbox, true
	case models.SectionWaiting:
		return models.StateWaiting, true
	case models.SectionReview:
		return models.StateReview, true
	case models.SectionSomeday:
		return models.StateSomeday, true
	case models.SectionStandby:
		return models.StatePaused, true
	case models.SectionCompleted:
		return models.StateDone, true
	}
	return "", false
}
