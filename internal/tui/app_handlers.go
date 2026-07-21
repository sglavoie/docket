package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// saveOutlinerToggleState captures the current outliner's section/fold state
// into the per-project cache so it can be restored after recreation.
func (m *AppModel) saveOutlinerToggleState() {
	if m.outliner != nil && m.outliner.project != nil {
		id := m.outliner.project.ID
		m.outlinerToggleStates[id] = m.outliner.SaveToggleState()
		for len(m.outlinerToggleStates) > maxProjectLastViewEntries {
			for k := range m.outlinerToggleStates {
				if k != id {
					delete(m.outlinerToggleStates, k)
					break
				}
			}
		}
	}
}

// restoreOutlinerToggleState applies any cached section/fold state to the
// current outliner for its project.
func (m *AppModel) restoreOutlinerToggleState() {
	if m.outliner != nil && m.outliner.project != nil {
		if state, ok := m.outlinerToggleStates[m.outliner.project.ID]; ok {
			m.outliner.RestoreToggleState(state)
		}
	}
}

// Presentation filters and layout are owned by their live view models. These
// navigation hooks retain only the in-memory fold cache used when a project
// view is recreated during the same session.
func (m *AppModel) saveOutlinerFilterState() {
	m.saveOutlinerToggleState()
}

func (m *AppModel) saveOutlinerFilterStateFrom(o *OutlinerModel) {
	if o == nil || o.project == nil {
		return
	}
	m.outlinerToggleStates[o.project.ID] = o.SaveToggleState()
}

func (m *AppModel) restoreOutlinerFilterState() {
	m.restoreOutlinerToggleState()
}

func (m *AppModel) restoreOutlinerFilterStateTo(o *OutlinerModel) {
	if o == nil || o.project == nil {
		return
	}
	if state, ok := m.outlinerToggleStates[o.project.ID]; ok {
		o.RestoreToggleState(state)
	}
	o.rebuildRows()
}

// reloadAndRefreshProjectTree reloads the project from storage and refreshes the
// per-project outliner view. Preserves filter/sort state across the reload.
func (m *AppModel) reloadAndRefreshProjectTree() {
	if m.outliner != nil && m.currentProject != nil {
		m.switchToProject(m.currentProject, true)
	}
}

// applyTagsToTask applies selected tags to a single task as one asynchronous
// transaction through the BulkCoordinator, mirroring the multi-task
// applyTagsToTaskRefs sibling. Setting tags is an attribute mutation (no workflow
// rules run); the shell owns reload and toast via the batch result, and the
// durable write records through the workspace undo ledger.
func (m *AppModel) applyTagsToTask(task *models.Task, project *models.Project, selectedTags []string) tea.Cmd {
	if task == nil {
		return nil
	}
	cmd := m.secondaryFeatures.Bulk.SetTags([]string{task.ID}, selectedTags)
	return finalizeBatchResult(cmd, nil, fmt.Sprintf("Tagged #%d", task.ShortID))
}

// applyTagsToTaskRefs applies selected tags to multiple task refs as one
// asynchronous cross-project transaction through the BulkCoordinator. Setting
// tags is an attribute mutation (no workflow rules run); the shell owns reload
// and selection-clear via the batch result.
func (m *AppModel) applyTagsToTaskRefs(refs []TaskProjectRef, selectedTags []string) tea.Cmd {
	refs = validTaskProjectRefs(refs)
	taskIDs := taskIDsFromTaskRefs(refs)
	if len(taskIDs) == 0 {
		return nil
	}
	cmd := m.secondaryFeatures.Bulk.SetTags(taskIDs, selectedTags)
	return finalizeBatchResult(cmd, taskIDs, fmt.Sprintf("Tagged %d tasks", len(taskIDs)))
}

// processMessage routes a message through the app's handler chain.
// Used by CommandPaletteExecuteMsg to process the inner message synchronously.
func (m *AppModel) processMessage(msg tea.Msg) (bool, tea.Cmd) {
	// Handle QuitMsg directly since it's not in the handler chains
	if _, ok := msg.(QuitMsg); ok {
		m.saveProjectViewState()
		m.saveOutlinerFilterState()
		m.quitting = true
		return true, tea.Quit
	}
	if handled, cmd := m.handleProjectMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleTaskMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleViewMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleTagMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleAttrFilterMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleSectionMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleInboxMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleClipboardMessages(msg); handled {
		return true, cmd
	}
	return false, nil
}
