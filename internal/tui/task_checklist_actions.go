package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *TaskChecklistModel) saveItem() tea.Cmd {
	text := strings.TrimSpace(m.textInput.Value())
	if text == "" {
		m.err = fmt.Errorf("item text cannot be empty")
		return nil
	}

	// Mutate model state synchronously in Update() to avoid data races with View().
	m.resolveTask()
	m.task.Checklist = append(m.task.Checklist, models.ChecklistItem{
		Text: text,
		Done: false,
	})
	m.addMode = false
	m.textInput.Reset()
	m.err = nil
	m.cursor = len(m.task.Checklist) - 1

	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return ChecklistItemAddedMsg{Task: task}
	}
}

func (m *TaskChecklistModel) executeDelete() tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	m.resolveTask()
	if m.cursor < 0 || m.cursor >= len(m.task.Checklist) {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("invalid index")} }
	}
	m.task.Checklist = append(m.task.Checklist[:m.cursor], m.task.Checklist[m.cursor+1:]...)
	if m.cursor >= len(m.task.Checklist) && m.cursor > 0 {
		m.cursor--
	}

	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return ChecklistItemDeletedMsg{Task: task}
	}
}

func (m *TaskChecklistModel) startEditItem() tea.Cmd {
	item := m.task.Checklist[m.cursor]
	m.editMode = true
	m.editIndex = m.cursor
	m.editTextInput.SetValue(item.Text)
	m.editTextInput.Focus()
	return textarea.Blink
}

func (m *TaskChecklistModel) saveEditedItem() tea.Cmd {
	text := strings.TrimSpace(m.editTextInput.Value())
	if text == "" {
		m.err = fmt.Errorf("item text cannot be empty")
		return nil
	}

	// Mutate model state synchronously in Update() to avoid data races with View().
	m.resolveTask()
	if m.editIndex < 0 || m.editIndex >= len(m.task.Checklist) {
		m.editMode = false
		m.editTextInput.Reset()
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("checklist item no longer exists")} }
	}
	m.task.Checklist[m.editIndex].Text = text
	m.editMode = false
	m.editTextInput.Reset()
	m.err = nil

	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return ChecklistItemEditedMsg{Task: task}
	}
}

func (m *TaskChecklistModel) toggleItem() tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	m.resolveTask()
	if m.cursor < 0 || m.cursor >= len(m.task.Checklist) {
		return nil
	}
	m.task.Checklist[m.cursor].Done = !m.task.Checklist[m.cursor].Done

	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return ChecklistItemToggledMsg{Task: task}
	}
}

func (m *TaskChecklistModel) reorderItem(from, to int) tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	m.resolveTask()
	if from < 0 || from >= len(m.task.Checklist) || to < 0 || to >= len(m.task.Checklist) {
		return nil
	}
	m.task.Checklist[from], m.task.Checklist[to] = m.task.Checklist[to], m.task.Checklist[from]
	m.cursor = to

	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return ChecklistItemEditedMsg{Task: task}
	}
}
