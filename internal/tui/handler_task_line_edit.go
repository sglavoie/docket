package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func (m *AppModel) handleTaskLineEditFailed(msg coordinator.TaskLineEditFailedMsg) tea.Cmd {
	m.err = msg.Err
	message := "task edit failed"
	if msg.Err != nil {
		message = msg.Err.Error()
	}
	if out, ok := m.outlinerPrimary(); ok && out != nil && out.quickEdit.IsActive() {
		task := out.quickEdit.GetTask()
		project := out.quickEdit.project
		if task != nil && project != nil && taskref.For(task, project) == msg.Source {
			out.quickEdit.RejectCommit(message)
		}
	}
	return status.StatusTimeout(5 * time.Second)
}

func (m *AppModel) handleTaskLineEdited(msg coordinator.TaskLineEditedMsg) tea.Cmd {
	m.err = nil
	out, ok := m.outlinerPrimary()
	if ok && out != nil && out.quickEdit.IsActive() {
		task := out.quickEdit.GetTask()
		project := out.quickEdit.project
		if task != nil && project != nil && taskref.For(task, project) == msg.Source {
			out.quickEdit.Exit()
		}
	}

	if !msg.Result.Changed {
		m.setInfoStatus("No task changes")
		return status.DefaultStatusTimeout()
	}
	for _, projectID := range msg.Result.AffectedProjectIDs {
		m.reloadProjectByID(projectID)
		m.reloadCrossProjectOutlinerProject(projectID)
	}
	if m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	m.refreshLoadedTaskWorkspace()

	recordedOrigin := false
	var jumpCmd tea.Cmd
	if ok && out != nil {
		final := taskref.TaskIdentity{
			ProjectID: msg.Result.Task.ProjectID,
			TaskID:    msg.Result.Task.TaskID,
		}
		jumpCmd, recordedOrigin = out.finishTaskLineEditRelocation(outlinerRelocationFrom(msg.Relocation), final)
	}

	summary := fmt.Sprintf("Updated #%d %s", msg.Result.ShortID, msg.Result.Title)
	if msg.Result.MovedDescendantCount > 0 {
		summary = fmt.Sprintf("Updated #%d and moved %d descendant(s)", msg.Result.ShortID, msg.Result.MovedDescendantCount)
	}
	saveCmd := m.handleSaveComplete(status.SaveCompleteMsg{})
	feedbackCmd := m.setRecoveryFeedback(summary, msg.Result.TransactionID, recordedOrigin)
	return tea.Batch(saveCmd, jumpCmd, feedbackCmd)
}
