package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleTaskUndoMessages handles task undo/redo messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskUndoMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case UndoPerformedMsg:
		return m.handleUndoPerformed(msg)

	case RedoPerformedMsg:
		return m.handleRedoPerformed(msg)
	}
	return false, nil
}

// handleUndoPerformed handles the UndoPerformedMsg.
func (m *AppModel) handleUndoPerformed(msg UndoPerformedMsg) (bool, tea.Cmd) {
	undoMsg := fmt.Sprintf("Undid: %s", msg.Action)
	return m.handleUndoRedoPerformed(undoMsg)
}

// handleRedoPerformed handles the RedoPerformedMsg.
func (m *AppModel) handleRedoPerformed(msg RedoPerformedMsg) (bool, tea.Cmd) {
	redoMsg := fmt.Sprintf("Redo: %s", msg.Action)
	return m.handleUndoRedoPerformed(redoMsg)
}

func (m *AppModel) handleUndoRedoPerformed(statusMsg string) (bool, tea.Cmd) {
	m.clearToggleHistory()
	m.setSuccessStatus(statusMsg)
	cmds := m.refreshTaskViewsAfterSave()
	cmds = append(cmds, defaultStatusTimeout())
	return true, tea.Batch(cmds...)
}

func (m *AppModel) repairTaskProjectRefsAfterRestore(projects []*models.Project) error {
	runtimeErr := repairRuntimeTaskRefsAfterRestore(m.runtime)(projects)
	m.rewireTaskProjectRefsAfterRestore(projects)
	return runtimeErr
}

func (m *AppModel) rewireTaskProjectRefsAfterRestore(projects []*models.Project) {
	refs := make(map[string]TaskProjectRef)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task != nil && task.ID != "" {
				refs[task.ID] = TaskProjectRef{Task: task, Project: project}
			}
		}
	}

	for i := range m.focusQueue {
		if m.focusQueue[i].Task == nil {
			continue
		}
		if ref, ok := refs[m.focusQueue[i].Task.ID]; ok {
			m.focusQueue[i].Task = ref.Task
			m.focusQueue[i].Project = ref.Project
		}
	}
	for i := range m.dailyPlan {
		if m.dailyPlan[i].Task == nil {
			continue
		}
		if ref, ok := refs[m.dailyPlan[i].Task.ID]; ok {
			m.dailyPlan[i].Task = ref.Task
			m.dailyPlan[i].Project = ref.Project
		}
	}
	for i := range m.recentTaskVisits {
		if m.recentTaskVisits[i].Task == nil {
			continue
		}
		if ref, ok := refs[m.recentTaskVisits[i].Task.ID]; ok {
			m.recentTaskVisits[i].Task = ref.Task
			m.recentTaskVisits[i].Project = ref.Project
		}
	}
}
