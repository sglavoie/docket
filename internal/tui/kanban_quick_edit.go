package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
)

// handleQuickEditKey handles the E key for quick edit mode.
func (m *KanbanViewModel) handleQuickEditKey() tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	return m.quickEdit.Enter(task, false)
}

// handleRenameKey handles the r key for rename mode.
func (m *KanbanViewModel) handleRenameKey() tea.Cmd {
	task := m.getSelectedTask()
	if task == nil {
		return nil
	}

	return m.quickEdit.Enter(task, true)
}

// handleQuickEditInput handles keyboard input while in quick edit mode.
func (m *KanbanViewModel) handleQuickEditInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		// Cancel quick edit
		m.quickEdit.Exit()
		m.statusMessage = ""
		return m, nil

	case "enter":
		// Confirm quick edit
		return m.confirmQuickEdit()

	default:
		// Update text input
		var cmd tea.Cmd
		input, cmd := m.quickEdit.Update(msg)
		m.quickEdit.UpdateInput(input)

		// Update live preview
		task := m.quickEdit.GetTask()
		if task != nil {
			if m.quickEdit.IsRenameOnly() {
				// Rename mode: just use input as title
				preview := &quickEditResult{
					title:    m.quickEdit.GetInput().Value(),
					titleSet: true,
				}
				m.quickEdit.SetPreview(preview)
			} else {
				// Full quick edit: parse modifiers
				preview, _ := parseQuickEditForPreview(m.quickEdit.GetInput().Value(), task, m.Project)
				m.quickEdit.SetPreview(preview)
			}
		}

		return m, cmd
	}
}

// confirmQuickEdit applies the quick edit changes and saves.
func (m *KanbanViewModel) confirmQuickEdit() (tea.Model, tea.Cmd) {
	task := m.quickEdit.GetTask()
	if task == nil {
		m.quickEdit.Exit()
		return m, nil
	}

	taskID := task.ID
	inputValue := m.quickEdit.GetInput().Value()

	if m.quickEdit.IsRenameOnly() {
		// Rename mode: just update title
		newTitle := strings.TrimSpace(inputValue)
		if newTitle == "" {
			m.statusMessage = "Title cannot be empty"
			return m, nil
		}

		prevStates := captureTaskStates(m.Project.Tasks)
		if !m.Project.MutateTask(taskID, func(t *models.Task) {
			t.Title = newTitle
		}) {
			m.quickEdit.Exit()
			return m, nil
		}
		m.quickEdit.Exit()
		m.statusMessage = "Renamed"
		m.lastModifiedTaskID = taskID

		return m, m.saveWithStatusChange(prevStates, task)
	}

	// Re-lookup task for parseQuickEdit which needs current task state
	task = m.Project.GetTaskByID(taskID)
	if task == nil {
		m.quickEdit.Exit()
		return m, nil
	}

	// Full quick edit: parse and apply changes
	result, err := parseQuickEdit(inputValue, task, m.Project)
	if err != nil {
		m.statusMessage = err.Error()
		return m, nil
	}

	// Check if there are actual changes
	original := m.quickEdit.GetOriginal()
	if original != nil && !result.hasChanges(original) {
		m.quickEdit.Exit()
		m.statusMessage = "No changes"
		return m, nil
	}

	prevStates := captureTaskStates(m.Project.Tasks)
	m.Project.MutateTask(taskID, func(t *models.Task) {
		applyQuickEdit(t, m.Project, result)
	})
	m.quickEdit.Exit()
	m.statusMessage = "Updated"
	m.lastModifiedTaskID = taskID

	return m, m.saveWithStatusChange(prevStates, task)
}

// renderQuickEditBar renders the quick edit input bar at the bottom of the view.
func (m *KanbanViewModel) renderQuickEditBar() string {
	if m.quickEdit == nil || !m.quickEdit.IsActive() {
		return ""
	}

	var lines []string

	// Mode label
	mode := "Quick Edit"
	if m.quickEdit.IsRenameOnly() {
		mode = "Rename"
	}
	modeLabel := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(mode + ":")

	// Input field
	inputLine := modeLabel + " " + m.quickEdit.GetInput().View()
	lines = append(lines, inputLine)

	// Preview line (only for full quick edit mode)
	if !m.quickEdit.IsRenameOnly() {
		preview := m.quickEdit.GetPreview()
		original := m.quickEdit.GetOriginal()
		if preview != nil && original != nil {
			previewStr := renderQuickEditPreview(preview, original)
			if previewStr != "" {
				previewLine := MutedStyle.Render(previewStr)
				lines = append(lines, previewLine)
			}
		}
	}

	// Help hint
	helpHint := MutedStyle.Render("Enter=save  Esc=cancel")
	if !m.quickEdit.IsRenameOnly() {
		helpHint = MutedStyle.Render("!1-5=priority #tag @date /section $status | Enter=save Esc=cancel")
	}
	lines = append(lines, helpHint)

	return strings.Join(lines, "\n")
}

// getQuickEditHeight returns the height needed for quick edit bar.
func (m *KanbanViewModel) getQuickEditHeight() int {
	if m.quickEdit == nil || !m.quickEdit.IsActive() {
		return 0
	}
	if m.quickEdit.IsRenameOnly() {
		return 2 // input + help
	}
	return 3 // input + preview + help
}

// HasTextInput implements ViewCapabilities. Returns true when in search, quick add, or quick edit mode.
func (m *KanbanViewModel) HasTextInput() bool {
	return m.searchMode || m.quickAddMode || (m.quickEdit != nil && m.quickEdit.IsActive()) || m.scoreFilterInput.IsActive()
}
