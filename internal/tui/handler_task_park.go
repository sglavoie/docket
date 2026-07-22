package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
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
	markParkPrompted(taskIdentityFor(msg.Task, msg.Project))

	// Commit through the application layer. The Park/Unpark use cases load a
	// fresh snapshot from disk, re-apply the mutation, and commit one
	// ledger-undoable ChangeSet synchronously, so the tasks.md write completes
	// before popView's ViewDidAppearMsg triggers reloadCrossProjects — an async
	// save would race that reload and could replace the parked task with unparked
	// disk state, silently dropping the park.
	res, err := m.applyParkMutation(msg)
	if err != nil {
		m.err = fmt.Errorf("park save failed: %w", err)
		return true, popCmd
	}
	m.reloadProjectByID(res.ProjectID)

	refreshCmd := m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: parkNoteStatusMessage(msg)})
	return true, tea.Batch(popCmd, refreshCmd)
}

// applyParkMutation commits a park or unpark for the task in msg through the
// application layer, returning the mutation result. Unpark discards the note;
// Park stores it (preserving any prior note as a timestamped comment).
func (m *AppModel) applyParkMutation(msg ParkNoteDoneMsg) (app.TaskResult, error) {
	locator := taskIdentityFor(msg.Task, msg.Project).Locator()
	if msg.Unparked {
		return m.appService.UnparkByLocator(locator)
	}
	return m.appService.ParkByLocator(locator, msg.Note)
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
