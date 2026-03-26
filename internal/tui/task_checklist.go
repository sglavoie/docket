package tui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// TaskChecklistModel displays and manages checklist items for a task.
type TaskChecklistModel struct {
	BaseView
	InteractiveViewState
	task    *models.Task
	project *models.Project

	// Add mode
	addMode   bool
	textInput textarea.Model

	// Edit mode
	editMode      bool
	editIndex     int
	editTextInput textarea.Model

	// Editor config
	editor string
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
	m.BaseView.SetSize(width, height)
	cardWidth := min(width-4, 80)
	contentWidth := cardWidth - 6 // card border + padding
	m.textInput.SetWidth(contentWidth)
	m.editTextInput.SetWidth(contentWidth)
}

func (m *TaskChecklistModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	var cmd tea.Cmd

	// Handle confirmation mode
	if m.confirm.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			action := m.confirm.Action()
			if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
				if confirmed {
					switch action {
					case "delete":
						return m, m.executeDelete()
					case "discard-add":
						m.addMode = false
						m.textInput.Reset()
						return m, nil
					case "discard-edit":
						m.editMode = false
						m.editTextInput.Reset()
						return m, nil
					}
				}
				m.ClearError()
				return m, nil
			}
		}
		return m, nil
	}

	// Handle edit mode
	if m.editMode {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "esc":
				if m.editIndex < len(m.task.Checklist) && strings.TrimSpace(m.editTextInput.Value()) != m.task.Checklist[m.editIndex].Text {
					m.confirm.Show("discard-edit", "Discard changes?")
					return m, nil
				}
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
		case tea.KeyPressMsg:
			switch msg.String() {
			case "esc":
				if strings.TrimSpace(m.textInput.Value()) != "" {
					m.confirm.Show("discard-add", "Discard item?")
					return m, nil
				}
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
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.WindowSizeMsg:
		m.BaseView.SetSize(msg.Width, msg.Height)

	case tea.KeyPressMsg:
		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			_, cmd := handleGotoTopPrefix(msg.String(), &m.cursor, &m.cursorFlash)
			return m, cmd
		}

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
			m.pendingPrefix = "g"
			return m, nil

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

		case "space", "enter":
			// Toggle done/undone
			if len(m.task.Checklist) > 0 && m.cursor < len(m.task.Checklist) {
				return m, m.toggleItem()
			}
		}
	}

	return m, nil
}

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
	if m.cursor < 0 || m.cursor >= len(m.task.Checklist) {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("invalid index")} }
	}

	// Mutate model state synchronously in Update() to avoid data races with View().
	m.resolveTask()
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
	if m.cursor < 0 || m.cursor >= len(m.task.Checklist) {
		return nil
	}

	// Mutate model state synchronously in Update() to avoid data races with View().
	m.resolveTask()
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

	editor, editorErr := external.ResolveEditor(m.editor)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return errorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

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

		// Bounds check after re-resolve (checklist may have changed)
		if editIndex < 0 || editIndex >= len(task.Checklist) {
			return ErrorMsg{Err: fmt.Errorf("checklist item no longer exists")}
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

	editor, editorErr := external.ResolveEditor(m.editor)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return errorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

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

	editor, editorErr := external.ResolveEditor(m.editor)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return errorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

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

		// Bounds check after re-resolve (checklist may have changed)
		if editIndex < 0 || editIndex >= len(task.Checklist) {
			return ErrorMsg{Err: fmt.Errorf("checklist item no longer exists")}
		}

		// Update item (preserve done state)
		task.Checklist[editIndex].Text = newText

		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return ChecklistItemEditedMsg{Task: task}
	})
}

func (m *TaskChecklistModel) View() tea.View {
	if m.task == nil {
		return tea.NewView("No task selected")
	}

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
	b.WriteString(renderStatusBarFooter(TitleStyle.Render(checklistTitle), MutedStyle.Render(progressStr), m.Width()))
	b.WriteString("\n\n")

	contentWidth := m.Width() - 4

	// Edit mode input
	if m.editMode {
		b.WriteString(FieldLabelFocusedStyle.Render(fmt.Sprintf("Edit Item [%d]:", m.editIndex)))
		b.WriteString("\n\n")
		b.WriteString(m.editTextInput.View())
		b.WriteString("\n\n")
		b.WriteString(renderStatusBarFooter(HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"), "", m.Width()))
		return tea.NewView(b.String())
	}

	// Add mode input
	if m.addMode {
		b.WriteString(FieldLabelFocusedStyle.Render("Add Item:"))
		b.WriteString("\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(renderStatusBarFooter(HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"), "", m.Width()))
		return tea.NewView(b.String())
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
		availableHeight := m.Height() - headerLines
		maxVisible := max(availableHeight/2, 5)

		scrollOffset := calcScrollOffset(m.cursor, maxVisible)

		maxLen := max(contentWidth-8, 1)

		for i := scrollOffset; i < len(m.task.Checklist) && i < scrollOffset+maxVisible; i++ {
			item := m.task.Checklist[i]

			cursor := "  "
			if i == m.cursor {
				cursor = "→ "
			}

			checkbox := "[ ]"
			if item.Done {
				checkbox = "[x]"
			}

			var entry strings.Builder
			entry.WriteString(cursor)

			text := strings.ReplaceAll(item.Text, "\n", " ")
			text = truncateToVisualWidth(text, maxLen)

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
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(helpText, m.Width())))

	return tea.NewView(b.String())
}

// HasTextInput returns true when add/edit mode is active.
func (m *TaskChecklistModel) HasTextInput() bool {
	return m.addMode || m.editMode
}

// ShouldCaptureEsc returns true when in add/edit, delete confirmation, or pending prefix modes.
func (m *TaskChecklistModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.addMode || m.editMode
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *TaskChecklistModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			"g": NavigationPrefixHints,
		},
	}
}

func (m *TaskChecklistModel) StatusBarLabel() string { return "Checklist" }
