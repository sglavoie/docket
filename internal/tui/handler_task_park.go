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
	// Save synchronously so the tasks.md write completes before popView's
	// ViewDidAppearMsg triggers reloadCrossProjects. That reload calls
	// storage.LoadProject on the SAME shared *models.Project pointer this save
	// serializes (cross-project loadedProjects are the config's own pointers).
	// An async save here races the reload, which can replace the parked task
	// with unparked disk state before serialization, silently dropping the
	// park. Every other cross-project mutation already saves synchronously.
	if err := m.persister.SaveWithBackupSyncOptions(
		msg.Project,
		m.taskMutationSaveOptions(msg.Task, action),
	); err != nil {
		m.err = fmt.Errorf("park save failed: %w", err)
		return true, popCmd
	}

	refreshCmd := m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: parkNoteStatusMessage(msg)})
	return true, tea.Batch(popCmd, refreshCmd)
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
