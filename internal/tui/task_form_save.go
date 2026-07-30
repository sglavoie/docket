package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// handleSave is the Ctrl+S entry point. It commits pending text inputs,
// validates, and dispatches to saveCreate or saveEdit.
func (m *TaskFormModel) handleSave() tea.Cmd {
	m.commitAllTextInputs()
	m.err = nil

	// Validate title (Basics tab, field 0).
	if err := models.ValidateTaskTitle(m.draft.title); err != nil {
		m.err = err
		m.activeTab = formTabBasics
		m.focusIndex = 0
		m.syncInputFocus()
		return nil
	}

	// Parse due date from text input (Schedule tab, field 0).
	if err := m.parseDraftDueDate(); err != nil {
		m.err = err
		m.activeTab = formTabSchedule
		m.focusIndex = 0
		m.syncInputFocus()
		return nil
	}

	if m.mode == FormModeCreate {
		return m.saveCreate()
	}
	return m.saveEdit()
}

// commitAllTextInputs flushes every live textinput/textarea value into the
// draft regardless of which field is focused. Safe to call multiple times.
func (m *TaskFormModel) commitAllTextInputs() {
	m.draft.title = strings.TrimSpace(strings.ReplaceAll(m.titleInput.Value(), "\n", " "))
	m.draft.dueInput = m.dueTextInput.Value()
}

// parseDraftDueDate parses draft.dueInput and stores the result in draft.dueDate.
// Empty / "none" / "clear" clears the due date. Returns an error for
// unparseable input. Supports optional time-of-day suffixes (e.g. "tomorrow 5pm").
func (m *TaskFormModel) parseDraftDueDate() error {
	value := strings.TrimSpace(m.draft.dueInput)
	if value == "" || strings.EqualFold(value, "none") || strings.EqualFold(value, "clear") {
		m.draft.dueDate = nil
		return nil
	}
	date, err := util.ParseRelativeDateWithConfig(value, "sun")
	if err != nil {
		return err
	}
	m.draft.dueDate = &date
	return nil
}

// saveCreate builds an AddTaskRequest from the draft and routes creation through
// the task-form coordinator, which commits the new task and any subtasks as one
// atomic ChangeSet. The returned Cmd emits a
// TaskCreatedMsg carrying the committed result; the shell reloads and re-resolves
// the task pointers for navigation, status, and open-after.
func (m *TaskFormModel) saveCreate() tea.Cmd {
	state := creationStateFromPreset(m.draft.state)

	// Build the subtask request list (blank titles skipped) and map the draft
	// open-after index onto the created (non-blank) subtask index.
	var subReqs []app.AddSubtaskRequest
	openCreatedIdx := -1
	if m.mode == FormModeCreate {
		wantOpen := m.subtaskOpenDetail || m.subtaskOpenContent
		created := 0
		for i, st := range m.draft.subtasks {
			title := strings.TrimSpace(st.title)
			if title == "" {
				continue
			}
			subReqs = append(subReqs, app.AddSubtaskRequest{Title: title})
			if wantOpen && i == m.subtaskOpenIdx {
				openCreatedIdx = created
			}
			created++
		}
	}

	req := app.AddTaskRequest{
		ProjectID:         m.project.ID,
		Title:             m.draft.title,
		Priority:          m.draft.priority,
		Tags:              append([]string{}, m.draft.tags...),
		Description:       strings.TrimSpace(m.draft.desc),
		State:             state,
		Recurrence:        m.draft.recurrence,
		ParentID:          m.draft.parentID,
		ChecklistItems:    append([]models.ChecklistItem(nil), m.draft.checklist...),
		DueDate:           m.draft.dueDate,
		Pinned:            m.draft.pinned,
		Subtasks:          subReqs,
		PlanningArtifacts: m.presetPlan,
	}

	openContent, openDetail := false, false
	if openCreatedIdx >= 0 {
		openContent = m.subtaskOpenContent
		openDetail = m.subtaskOpenDetail
		m.subtaskOpenContent = false
		m.subtaskOpenDetail = false
	}

	return m.taskForm.CreateForm(req, openCreatedIdx, openContent, openDetail)
}

// saveEdit builds a full-field EditRequest from the draft and routes the edit
// through the task-form coordinator, which commits the change as one atomic
// ChangeSet. The returned Cmd emits a TaskUpdatedMsg carrying the committed
// result; the shell reloads and re-resolves the live task pointer.
func (m *TaskFormModel) saveEdit() tea.Cmd {
	form := &app.FormEdit{
		Title:       m.draft.title,
		Priority:    m.draft.priority,
		ParentID:    m.draft.parentID,
		Description: strings.TrimSpace(m.draft.desc),
		Tags:        append([]string{}, m.draft.tags...),
		DueDate:     m.draft.dueDate,
		Pinned:      m.draft.pinned,
		Recurrence:  m.draft.recurrence,
		Checklist:   append([]models.ChecklistItem(nil), m.draft.checklist...),
	}

	state := m.draft.state
	return m.taskForm.EditForm(
		taskIdentityFor(m.task, m.project),
		app.EditRequest{Form: form, WorkflowState: &state},
	)
}

// SetTaskFormCoordinator injects the shared task-form coordinator (taskFormWireable).
func (m *TaskFormModel) SetTaskFormCoordinator(c TaskFormCoordinator) { m.taskForm = c }
