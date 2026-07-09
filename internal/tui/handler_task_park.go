package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleTaskParkMessages handles task parking messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskParkMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowParkInputMsg:
		return m.handleShowParkInput(msg)

	case ParkNoteDoneMsg:
		return m.handleParkNoteDone(msg)
	}
	return false, nil
}

func (m *AppModel) handleShowParkInput(msg ShowParkInputMsg) (bool, tea.Cmd) {
	if msg.Task != nil && msg.Project != nil {
		m.showView(ViewParkInput, NewParkInputModel(msg.Task, msg.Project))
	}
	return true, nil
}

func (m *AppModel) handleParkNoteDone(msg ParkNoteDoneMsg) (bool, tea.Cmd) {
	if m.pendingParkNav != nil {
		return m.finishPendingParkPrompt(msg)
	}

	popCmd := m.popView()
	if msg.Skipped || msg.Task == nil || msg.Project == nil {
		return true, popCmd
	}
	if fresh := msg.Project.GetTaskByID(msg.Task.ID); fresh != nil {
		msg.Task = fresh
	}
	markParkPrompted(msg.Task.ID)

	action := "Park Task"
	if msg.Unparked {
		action = "Unpark Task"
	}
	// Rebuild now for immediate visual feedback; the async save will trigger
	// the full cross-project reload after persistence completes.
	m.refreshCrossProjectParkRows(msg.Project)
	saveCmd := SaveProjectAsyncWithOptions(
		msg.Project,
		parkNoteStatusMessage(msg),
		m.taskMutationSaveOptions(msg.Task, action),
	)
	return true, tea.Batch(popCmd, saveCmd)
}

func (m *AppModel) refreshCrossProjectParkRows(project *models.Project) {
	if project == nil || m.baseProjectView() != ViewCrossProjectOutliner {
		return
	}
	cross := m.activeCrossProjectOutliner()
	if cross == nil || cross.project != nil {
		return
	}
	replaced := false
	for i, loaded := range cross.loadedProjects {
		if loaded != nil && loaded.ID == project.ID {
			cross.loadedProjects[i] = project
			replaced = true
			break
		}
	}
	if !replaced {
		return
	}
	cross.rebuildRows()
}

func parkNoteStatusMessage(msg ParkNoteDoneMsg) string {
	verb := "Parked"
	if msg.Unparked {
		verb = "Unparked"
	}
	label := parkTaskStatusLabel(msg.Task)
	if !msg.Unparked {
		if note := strings.TrimSpace(msg.Note); note != "" {
			return fmt.Sprintf("%s %s: %s", verb, label, note)
		}
	}
	return fmt.Sprintf("%s %s", verb, label)
}

func parkTaskStatusLabel(task *models.Task) string {
	if task == nil {
		return "task"
	}
	if task.ShortID > 0 {
		return fmt.Sprintf("task #%d", task.ShortID)
	}
	if strings.TrimSpace(task.Title) != "" {
		return fmt.Sprintf("task %q", task.Title)
	}
	return "task"
}
