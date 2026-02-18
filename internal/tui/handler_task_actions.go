package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// handleCommandPaletteTaskAction handles inline task actions from the command palette.
// It applies status changes (complete, start, reopen) and saves the project
// through the full workflow pipeline (auto-unblock, recurring, workflow rules).
func (m *AppModel) handleCommandPaletteTaskAction(msg CommandPaletteTaskActionMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.Project == nil {
		return true, nil
	}

	current := msg.Project.GetTaskByID(msg.Task.ID)
	if current == nil {
		return true, errorCmd("task no longer exists in project")
	}

	// Map action name to target status
	statusMap := map[string]models.TaskStatus{
		"complete": models.StatusDone,
		"start":    models.StatusInProgress,
		"block":    models.StatusBlocked,
		"cancel":   models.StatusCanceled,
		"reopen":   models.StatusTodo,
		"review":   models.StatusReview,
	}
	targetStatus, ok := statusMap[msg.Action]
	if !ok {
		return true, nil
	}
	if msg := current.CanTransitionTo(targetStatus); msg != "" {
		return true, errorCmd("%s", msg)
	}

	prevStates := captureTaskStates(msg.Project.Tasks)
	current.ApplyTransition(targetStatus)

	successMsg := fmt.Sprintf("Task %s: %s", msg.Action, current.Title)
	opts := WorkflowSaveOptions{
		Persister:     m.persister,
		Project:       msg.Project,
		Config:        m.config,
		ConfigManager: m.configManager,
		PrevStates:    prevStates,
	}
	return true, tea.Batch(
		startLoading("Saving..."),
		func() tea.Msg {
			if err := persistWithWorkflow(opts); err != nil {
				return SaveErrorMsg{Err: err}
			}
			return SaveCompleteMsg{SuccessMessage: successMsg}
		},
	)
}

// handleInlineTaskCreate creates a new task from the command palette inline creation.
// Saves through the full workflow pipeline (auto-unblock, recurring, workflow rules).
func (m *AppModel) handleInlineTaskCreate(msg InlineTaskCreateMsg) (bool, tea.Cmd) {
	if msg.Project == nil || msg.Title == "" {
		return true, nil
	}

	// Ensure project is loaded
	if err := m.persister.Load(msg.Project); err != nil {
		return true, errorCmd("failed to load project: %w", err)
	}

	// Ensure Inbox section exists
	if !msg.Project.HasSection(models.SectionInbox) {
		msg.Project.EnsureSection(models.SectionInbox)
		msg.Project.SetSectionColor(models.SectionInbox, "red")
	}

	// Create the task
	task := &models.Task{
		ID:        uuid.New().String(),
		ShortID:   msg.Project.GetNextShortID(),
		Type:      models.TaskTypeTask,
		Title:     msg.Title,
		Status:    models.StatusTodo,
		Priority:  msg.Priority,
		Section:   models.SectionInbox,
		DueDate:   msg.DueDate,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Add tags
	for _, tag := range msg.Tags {
		task.AddTag(tag)
	}

	// Add to project at the beginning so it's prominently visible
	msg.Project.Tasks = append([]*models.Task{task}, msg.Project.Tasks...)
	msg.Project.BuildTaskTree()

	successMsg := fmt.Sprintf("Created: %s", msg.Title)
	opts := WorkflowSaveOptions{
		Persister:     m.persister,
		Project:       msg.Project,
		Config:        m.config,
		ConfigManager: m.configManager,
	}
	return true, tea.Batch(
		startLoading("Saving..."),
		func() tea.Msg {
			if err := persistWithWorkflow(opts); err != nil {
				return SaveErrorMsg{Err: err}
			}
			return SaveCompleteMsg{SuccessMessage: successMsg}
		},
	)
}
