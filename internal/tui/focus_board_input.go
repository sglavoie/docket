package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// handleAddInput handles keyboard input in add mode.
func (m *FocusBoardModel) handleAddInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Let autocomplete handle keys first when active
	if m.handleBoardAutocompleteKey(key) {
		return m, nil
	}

	switch key {
	case "esc":
		m.addMode = false
		m.addInput.Blur()
		m.ac.Clear()
		return m, nil
	case "enter":
		if m.ac.IsActive() {
			return m, nil
		}
		value := strings.TrimSpace(m.addInput.Value())
		if value == "" {
			m.addMode = false
			m.addInput.Blur()
			m.ac.Clear()
			return m, nil
		}
		return m.createItemFromInput(value)
	default:
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
		m.refreshBoardAutocomplete()
		return m, cmd
	}
}

// handleEditInput handles keyboard input in edit mode.
func (m *FocusBoardModel) handleEditInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editMode = false
		m.editInput.Blur()
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.editInput.Value())
		m.editMode = false
		m.editInput.Blur()
		if value == "" {
			return m, nil
		}
		if err := models.ValidateTaskTitle(value); err != nil {
			m.setStatus(err.Error())
			return m, nil
		}
		if m.editIdx >= 0 && m.editIdx < len(m.filtered) {
			row := m.filtered[m.editIdx]
			// Commit the title edit through the application-layer single-task
			// coordinator; the shell's transaction-result handler reloads the
			// committed project and refreshes the board (refreshData).
			cmd := m.taskItem.Edit(app.EditRequest{TaskID: row.Task.ID, Title: &value})
			msg := cmd()
			if em, ok := msg.(TransactionErrorMsg); ok {
				m.setStatus("Failed to save: " + em.Err.Error())
				return m, nil
			}
			return m, func() tea.Msg { return msg }
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}
}

// SetCaptureCoordinator injects the shared capture coordinator (captureWireable).
func (m *FocusBoardModel) SetCaptureCoordinator(c CaptureCoordinator) { m.capture = c }

// SetTaskItemCoordinator injects the shared single-task coordinator so the board's
// inline title edit commits through the application layer (taskItemWireable).
func (m *FocusBoardModel) SetTaskItemCoordinator(c TaskItemCoordinator) { m.taskItem = c }

// createItemFromInput parses the add input and creates a task through the
// application-layer capture coordinator.
func (m *FocusBoardModel) createItemFromInput(input string) (tea.Model, tea.Cmd) {
	m.addMode = false
	m.addInput.Blur()
	m.ac.Clear()

	result := parseBoardAddInput(input)

	// Determine target project.
	var targetProject *models.Project

	if result.projectSlug != "" {
		for _, p := range m.acProjectCache {
			if p.Slug == result.projectSlug {
				targetProject = p
				break
			}
		}
	}

	if targetProject == nil {
		targetProject = m.runtime.SystemInboxProject()
		if targetProject == nil {
			m.setStatus("System Inbox is unavailable")
			return m, nil
		}
	}

	if err := models.ValidateTaskTitle(result.title); err != nil {
		m.setStatus("Invalid task: " + err.Error())
		return m, nil
	}

	// Non-GTD section input is preserved as a tag (custom section writes are
	// retired under the workflow-state model), matching Bulk Capture.
	section := result.section
	tags := result.tags
	if section != "" && !models.IsGTDSection(section) {
		tags = append(tags, section)
		section = ""
	}

	priority := 0
	if result.priority >= 1 && result.priority <= 5 {
		priority = result.priority
	}

	req := app.CaptureRequest{
		ProjectID:  targetProject.ID,
		Title:      result.title,
		Priority:   priority,
		Tags:       tags,
		RemoveTags: result.tagsToRemove,
		Section:    section,
		Due:        result.dueDate,
		URL:        result.url,
		Source:     result.source,
		State:      captureStateForStatus(result.status),
	}
	return m, m.capture.CaptureBoard(req)
}
