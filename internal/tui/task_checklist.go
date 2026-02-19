package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// TaskChecklistModel displays and manages checklist items for a task.
type TaskChecklistModel struct {
	task    *models.Task
	project *models.Project

	// Navigation state
	cursor int // Selected checklist item index

	// Add mode
	addMode   bool
	textInput textarea.Model

	// Edit mode
	editMode      bool
	editIndex     int
	editTextInput textarea.Model

	// Confirmation state
	confirm ConfirmDialog

	// Editor config
	editor string

	// Dimensions
	width  int
	height int

	// Error display
	err error
}

// NewTaskChecklistModel creates a new task checklist viewer.
func NewTaskChecklistModel(task *models.Task, project *models.Project) *TaskChecklistModel {
	ti := textarea.New()
	ti.Placeholder = "Enter checklist item..."
	ti.CharLimit = 500
	ti.SetHeight(2)
	ti.ShowLineNumbers = false

	editTi := textarea.New()
	editTi.Placeholder = "Edit checklist item..."
	editTi.CharLimit = 500
	editTi.SetHeight(2)
	editTi.ShowLineNumbers = false

	return &TaskChecklistModel{
		task:          task,
		project:       project,
		cursor:        0,
		addMode:       false,
		textInput:     ti,
		editTextInput: editTi,
	}
}

// NewTaskChecklistModelWithEditor creates a task checklist viewer with editor config.
func NewTaskChecklistModelWithEditor(task *models.Task, project *models.Project, editor string) *TaskChecklistModel {
	m := NewTaskChecklistModel(task, project)
	m.editor = editor
	return m
}

// resolveTask ensures m.task points to the task currently in m.project.Tasks.
func (m *TaskChecklistModel) resolveTask() {
	if t := m.project.GetTaskByID(m.task.ID); t != nil {
		m.task = t
	}
}

func (m *TaskChecklistModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskChecklistModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	cardWidth := width - 4
	if cardWidth > 80 {
		cardWidth = 80
	}
	contentWidth := cardWidth - 6 // card border + padding
	m.textInput.SetWidth(contentWidth)
	m.editTextInput.SetWidth(contentWidth)
}

func (m *TaskChecklistModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle confirmation mode
	if m.confirm.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if confirmed, handled := m.confirm.HandleKey(msg.String()); handled {
				if confirmed {
					return m, m.executeDelete()
				}
				m.err = nil
				return m, nil
			}
		}
		return m, nil
	}

	// Handle edit mode
	if m.editMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.editMode = false
				m.editTextInput.Reset()
				return m, nil
			case "ctrl+s":
				return m, m.saveEditedItem()
			case "ctrl+e":
				return m, m.editItemInExternalEditorFromEditMode()
			}
		}
		m.editTextInput, cmd = m.editTextInput.Update(msg)
		return m, cmd
	}

	// Handle add mode
	if m.addMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.addMode = false
				m.textInput.Reset()
				return m, nil
			case "ctrl+s":
				return m, m.saveItem()
			case "ctrl+e":
				return m, m.addItemInExternalEditor()
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	// Normal mode
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return CloseTaskChecklistMsg{} }

		case "j", "down":
			if len(m.task.Checklist) > 0 && m.cursor < len(m.task.Checklist)-1 {
				m.cursor++
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "g":
			m.cursor = 0

		case "G":
			if len(m.task.Checklist) > 0 {
				m.cursor = len(m.task.Checklist) - 1
			}

		case "a", "n":
			// Enter add mode
			m.addMode = true
			m.textInput.Focus()
			return m, textarea.Blink

		case "e":
			// Edit selected item
			if len(m.task.Checklist) > 0 && m.cursor < len(m.task.Checklist) {
				return m, m.startEditItem()
			}

		case "E":
			// Edit in external editor
			if len(m.task.Checklist) > 0 && m.cursor < len(m.task.Checklist) {
				return m, m.editInExternalEditor()
			}

		case "d", "x":
			// Request delete confirmation
			if len(m.task.Checklist) > 0 && m.cursor < len(m.task.Checklist) {
				m.confirm.Show("delete", fmt.Sprintf("Delete item [%d]?", m.cursor))
				return m, nil
			}

		case " ", "enter":
			// Toggle done/undone
			if len(m.task.Checklist) > 0 && m.cursor < len(m.task.Checklist) {
				return m, m.toggleItem()
			}
		}
	}

	return m, nil
}

func (m *TaskChecklistModel) saveItem() tea.Cmd {
	return func() tea.Msg {
		text := strings.TrimSpace(m.textInput.Value())
		if text == "" {
			m.err = fmt.Errorf("item text cannot be empty")
			return nil
		}

		m.resolveTask()
		m.task.Checklist = append(m.task.Checklist, models.ChecklistItem{
			Text: text,
			Done: false,
		})

		if err := saveProjectNoReload(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reset add mode
		m.addMode = false
		m.textInput.Reset()
		m.err = nil

		// Move cursor to the new item (last in list)
		m.cursor = len(m.task.Checklist) - 1

		return ChecklistItemAddedMsg{Task: m.task}
	}
}

func (m *TaskChecklistModel) executeDelete() tea.Cmd {
	return func() tea.Msg {
		if m.cursor < 0 || m.cursor >= len(m.task.Checklist) {
			return ErrorMsg{Err: fmt.Errorf("invalid index")}
		}

		// Remove item at cursor
		m.resolveTask()
		m.task.Checklist = append(m.task.Checklist[:m.cursor], m.task.Checklist[m.cursor+1:]...)

		if err := saveProjectNoReload(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Adjust cursor if needed
		if m.cursor >= len(m.task.Checklist) && m.cursor > 0 {
			m.cursor--
		}

		return ChecklistItemDeletedMsg{Task: m.task}
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
	return func() tea.Msg {
		text := strings.TrimSpace(m.editTextInput.Value())
		if text == "" {
			m.err = fmt.Errorf("item text cannot be empty")
			return nil
		}

		// Update the item text (preserve done state)
		m.resolveTask()
		m.task.Checklist[m.editIndex].Text = text

		if err := saveProjectNoReload(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reset edit mode
		m.editMode = false
		m.editTextInput.Reset()
		m.err = nil

		return ChecklistItemEditedMsg{Task: m.task}
	}
}

func (m *TaskChecklistModel) toggleItem() tea.Cmd {
	return func() tea.Msg {
		if m.cursor < 0 || m.cursor >= len(m.task.Checklist) {
			return nil
		}

		m.resolveTask()
		m.task.Checklist[m.cursor].Done = !m.task.Checklist[m.cursor].Done

		if err := saveProjectNoReload(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return ChecklistItemToggledMsg{Task: m.task}
	}
}

func (m *TaskChecklistModel) editInExternalEditor() tea.Cmd {
	item := m.task.Checklist[m.cursor]
	editIndex := m.cursor

	// Create temp file content
	content := fmt.Sprintf("# Checklist Item %d for #%d: %s\n\n", editIndex, m.task.ShortID, m.task.Title)
	content += "<!-- Edit the item text below. -->\n\n"
	content += item.Text

	// Create temp file
	tmpFile, err := os.CreateTemp("", "tt-checklist-*.md")
	if err != nil {
		m.err = fmt.Errorf("failed to create temp file: %w", err)
		return nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	// Determine editor
	editor := m.editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "code"
	}

	// Build command
	var c *exec.Cmd
	if editor == "code" {
		c = exec.Command(editor, "--wait", tmpPath)
	} else {
		c = exec.Command(editor, tmpPath)
	}

	task := m.task
	project := m.project

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		// Read modified content
		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		// Parse the edited content
		newText := util.ParseEditedCommentContent(string(modified))
		if newText == "" {
			return ErrorMsg{Err: fmt.Errorf("item text cannot be empty")}
		}

		// Check if changed
		if newText == item.Text {
			return nil // No changes
		}

		// Re-resolve task in case an async save replaced project.Tasks
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}

		// Update item (preserve done state)
		task.Checklist[editIndex].Text = newText

		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return ChecklistItemEditedMsg{Task: task}
	})
}

func (m *TaskChecklistModel) addItemInExternalEditor() tea.Cmd {
	// Get current textarea content
	currentText := strings.TrimSpace(m.textInput.Value())

	// Create temp file content
	content := fmt.Sprintf("# New checklist item for #%d: %s\n\n", m.task.ShortID, m.task.Title)
	content += "<!-- Enter the item text below. -->\n\n"
	content += currentText

	// Create temp file
	tmpFile, err := os.CreateTemp("", "tt-checklist-*.md")
	if err != nil {
		m.err = fmt.Errorf("failed to create temp file: %w", err)
		return nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	// Determine editor
	editor := m.editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "code"
	}

	// Build command
	var c *exec.Cmd
	if editor == "code" {
		c = exec.Command(editor, "--wait", tmpPath)
	} else {
		c = exec.Command(editor, tmpPath)
	}

	task := m.task
	project := m.project

	// Reset add mode before launching editor
	m.addMode = false
	m.textInput.Reset()

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		// Read modified content
		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		// Parse the edited content
		newText := util.ParseEditedCommentContent(string(modified))
		if newText == "" {
			return nil // Empty item, just return to normal mode
		}

		// Re-resolve task in case an async save replaced project.Tasks
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}

		// Add the new item
		task.Checklist = append(task.Checklist, models.ChecklistItem{
			Text: newText,
			Done: false,
		})

		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return ChecklistItemAddedMsg{Task: task}
	})
}

func (m *TaskChecklistModel) editItemInExternalEditorFromEditMode() tea.Cmd {
	item := m.task.Checklist[m.editIndex]
	editIndex := m.editIndex

	// Get current textarea content (may have been modified)
	currentText := strings.TrimSpace(m.editTextInput.Value())

	// Create temp file content
	content := fmt.Sprintf("# Checklist Item %d for #%d: %s\n\n", editIndex, m.task.ShortID, m.task.Title)
	content += "<!-- Edit the item text below. -->\n\n"
	content += currentText

	// Create temp file
	tmpFile, err := os.CreateTemp("", "tt-checklist-*.md")
	if err != nil {
		m.err = fmt.Errorf("failed to create temp file: %w", err)
		return nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	// Determine editor
	editor := m.editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "code"
	}

	// Build command
	var c *exec.Cmd
	if editor == "code" {
		c = exec.Command(editor, "--wait", tmpPath)
	} else {
		c = exec.Command(editor, tmpPath)
	}

	task := m.task
	project := m.project
	originalText := item.Text

	// Reset edit mode before launching editor
	m.editMode = false
	m.editTextInput.Reset()

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		// Read modified content
		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		// Parse the edited content
		newText := util.ParseEditedCommentContent(string(modified))
		if newText == "" {
			return ErrorMsg{Err: fmt.Errorf("item text cannot be empty")}
		}

		// Check if changed from original
		if newText == originalText {
			return nil // No changes
		}

		// Re-resolve task in case an async save replaced project.Tasks
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}

		// Update item (preserve done state)
		task.Checklist[editIndex].Text = newText

		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return ChecklistItemEditedMsg{Task: task}
	})
}

func (m *TaskChecklistModel) View() string {
	var b strings.Builder

	// Count completed
	completed := 0
	for _, item := range m.task.Checklist {
		if item.Done {
			completed++
		}
	}
	total := len(m.task.Checklist)

	checklistTitle := fmt.Sprintf("Checklist for: #%d: %s", m.task.ShortID, m.task.Title)
	progressStr := ""
	if total > 0 {
		progressStr = fmt.Sprintf("%d/%d completed", completed, total)
	}
	b.WriteString(renderFullscreenTitle(checklistTitle, progressStr))
	b.WriteString("\n\n")

	contentWidth := m.width - 4

	// Edit mode input
	if m.editMode {
		b.WriteString(FieldLabelFocusedStyle.Render(fmt.Sprintf("Edit Item [%d]:", m.editIndex)))
		b.WriteString("\n\n")
		b.WriteString(m.editTextInput.View())
		b.WriteString("\n\n")
		b.WriteString(renderStatusBarFooter(HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"), "", m.width))
		return b.String()
	}

	// Add mode input
	if m.addMode {
		b.WriteString(FieldLabelFocusedStyle.Render("Add Item:"))
		b.WriteString("\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(renderStatusBarFooter(HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"), "", m.width))
		return b.String()
	}

	// Delete confirmation
	if m.confirm.IsActive() && len(m.task.Checklist) > 0 && m.cursor < len(m.task.Checklist) {
		item := m.task.Checklist[m.cursor]
		preview := item.Text
		if len(preview) > 50 {
			preview = preview[:47] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")

		b.WriteString(ValidationErrorStyle.Bold(true).Render(fmt.Sprintf("Delete item [%d]?", m.cursor)))
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  \"%s\"", preview)))
		b.WriteString("\n\n")
		b.WriteString(renderStatusBarFooter(HelpStyle.Render("y: confirm delete · n/Esc: cancel"), "", m.width))
		return b.String()
	}

	// Error display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	// Checklist items
	if len(m.task.Checklist) == 0 {
		b.WriteString(MutedStyle.Render("No checklist items yet. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		headerLines := 6
		availableHeight := m.height - headerLines
		maxVisible := availableHeight / 2
		if maxVisible < 5 {
			maxVisible = 5
		}

		scrollOffset := calcScrollOffset(m.cursor, maxVisible)

		maxLen := contentWidth - 8
		if maxLen < 40 {
			maxLen = 40
		}

		for i := scrollOffset; i < len(m.task.Checklist) && i < scrollOffset+maxVisible; i++ {
			item := m.task.Checklist[i]

			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			checkbox := "[ ]"
			if item.Done {
				checkbox = "[x]"
			}

			var entry strings.Builder
			entry.WriteString(cursor)

			text := item.Text
			if len(text) > maxLen {
				text = text[:maxLen-3] + "..."
			}
			text = strings.ReplaceAll(text, "\n", " ")

			if i == m.cursor {
				entry.WriteString(BoldStyle.Render(checkbox + " "))
				if item.Done {
					entry.WriteString(MutedStyle.Render(text))
				} else {
					entry.WriteString(FieldLabelFocusedStyle.Render(text))
				}
			} else {
				entry.WriteString(MutedStyle.Render(checkbox + " "))
				if item.Done {
					entry.WriteString(MutedStyle.Render(text))
				} else {
					entry.WriteString(text)
				}
			}
			entry.WriteString("\n")

			b.WriteString(entry.String())
		}

		if len(m.task.Checklist) > maxVisible {
			remaining := len(m.task.Checklist) - scrollOffset - maxVisible
			if remaining > 0 {
				b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	helpText := "j/k: nav · Space: toggle · a: add · e: edit · E: external editor · d: delete · q: close"
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(helpText, m.width)))

	return b.String()
}

// HasTextInput returns true when add/edit mode is active.
func (m *TaskChecklistModel) HasTextInput() bool {
	return m.addMode || m.editMode
}

// ShouldCaptureEsc returns true when in add/edit or delete confirmation modes.
func (m *TaskChecklistModel) ShouldCaptureEsc() bool {
	return m.confirm.IsActive() || m.addMode || m.editMode
}
