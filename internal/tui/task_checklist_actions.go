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

	return commitTaskItemMutation(m.project, m.task, m.coordinator.AddChecklistItem(m.task.ID, text), func(fresh *models.Task) tea.Msg {
		return ChecklistItemAddedMsg{Task: fresh}
	})
}

func (m *TaskChecklistModel) executeDelete() tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	m.resolveTask()
	if m.cursor < 0 || m.cursor >= len(m.task.Checklist) {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("invalid index")} }
	}
	index := m.cursor
	m.task.Checklist = append(m.task.Checklist[:index], m.task.Checklist[index+1:]...)
	if m.cursor >= len(m.task.Checklist) && m.cursor > 0 {
		m.cursor--
	}

	return commitTaskItemMutation(m.project, m.task, m.coordinator.RemoveChecklistItem(m.task.ID, index), func(fresh *models.Task) tea.Msg {
		return ChecklistItemDeletedMsg{Task: fresh}
	})
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
	index := m.editIndex
	m.task.Checklist[index].Text = text
	m.editMode = false
	m.editTextInput.Reset()
	m.err = nil

	return commitTaskItemMutation(m.project, m.task, m.coordinator.EditChecklistItem(m.task.ID, index, text), func(fresh *models.Task) tea.Msg {
		return ChecklistItemEditedMsg{Task: fresh}
	})
}

func (m *TaskChecklistModel) toggleItem() tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	m.resolveTask()
	if m.cursor < 0 || m.cursor >= len(m.task.Checklist) {
		return nil
	}
	index := m.cursor
	m.task.Checklist[index].Done = !m.task.Checklist[index].Done

	return commitTaskItemMutation(m.project, m.task, m.coordinator.ToggleChecklistItem(m.task.ID, index), func(fresh *models.Task) tea.Msg {
		return ChecklistItemToggledMsg{Task: fresh}
	})
}

func (m *TaskChecklistModel) reorderItem(from, to int) tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	m.resolveTask()
	if from < 0 || from >= len(m.task.Checklist) || to < 0 || to >= len(m.task.Checklist) {
		return nil
	}
	m.task.Checklist[from], m.task.Checklist[to] = m.task.Checklist[to], m.task.Checklist[from]
	m.cursor = to

	return commitTaskItemMutation(m.project, m.task, m.coordinator.ReorderChecklistItem(m.task.ID, from, to), func(fresh *models.Task) tea.Msg {
		return ChecklistItemEditedMsg{Task: fresh}
	})
}
