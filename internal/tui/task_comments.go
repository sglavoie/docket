package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// TaskCommentsModel displays and manages comments for a task.
type TaskCommentsModel struct {
	task    *models.Task
	project *models.Project

	// Navigation state
	cursor int // Selected comment index

	// Add mode
	addMode   bool
	textInput textarea.Model

	// Edit mode
	editMode      bool
	editIndex     int
	editTextInput textarea.Model

	// Delete confirmation
	confirmDelete bool

	// Expanded view mode (show full comment)
	expandedView bool

	// Editor config
	editor string

	// Dimensions
	width  int
	height int

	// Error display
	err error
}

// NewTaskCommentsModel creates a new task comments viewer.
func NewTaskCommentsModel(task *models.Task, project *models.Project) *TaskCommentsModel {
	ti := textarea.New()
	ti.Placeholder = "Enter comment text..."
	ti.CharLimit = 2000
	ti.SetHeight(4)
	ti.ShowLineNumbers = false

	editTi := textarea.New()
	editTi.Placeholder = "Edit comment text..."
	editTi.CharLimit = 2000
	editTi.SetHeight(6)
	editTi.ShowLineNumbers = false

	return &TaskCommentsModel{
		task:          task,
		project:       project,
		cursor:        0,
		addMode:       false,
		textInput:     ti,
		editTextInput: editTi,
	}
}

// NewTaskCommentsModelWithEditor creates a task comments viewer with editor config.
func NewTaskCommentsModelWithEditor(task *models.Task, project *models.Project, editor string) *TaskCommentsModel {
	m := NewTaskCommentsModel(task, project)
	m.editor = editor
	return m
}

func (m *TaskCommentsModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskCommentsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.textInput.SetWidth(width - 4)
	m.editTextInput.SetWidth(width - 4)
}

func (m *TaskCommentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle expanded view mode
	if m.expandedView {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc", "enter", "q":
				m.expandedView = false
				return m, nil
			case "j", "down":
				// Allow scrolling in expanded view (future enhancement)
				return m, nil
			case "k", "up":
				return m, nil
			}
		}
		return m, nil
	}

	// Handle delete confirmation mode
	if m.confirmDelete {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				m.confirmDelete = false
				return m, m.executeDelete()
			case "n", "N", "esc":
				m.confirmDelete = false
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
				return m, m.saveEditedComment()
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
				return m, m.saveComment()
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
			return m, func() tea.Msg { return CloseTaskCommentsMsg{} }

		case "j", "down":
			if len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments)-1 {
				m.cursor++
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "g":
			m.cursor = 0

		case "G":
			if len(m.task.Comments) > 0 {
				m.cursor = len(m.task.Comments) - 1
			}

		case "a", "n":
			// Enter add mode
			m.addMode = true
			m.textInput.Focus()
			return m, textarea.Blink

		case "e":
			// Edit selected comment
			if len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments) {
				return m, m.startEditComment()
			}

		case "E":
			// Edit in external editor
			if len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments) {
				return m, m.editInExternalEditor()
			}

		case "d", "x":
			// Request delete confirmation
			if len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments) {
				m.confirmDelete = true
				return m, nil
			}

		case "enter":
			// Expand selected comment to view full text
			if len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments) {
				m.expandedView = true
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *TaskCommentsModel) saveComment() tea.Cmd {
	return func() tea.Msg {
		text := strings.TrimSpace(m.textInput.Value())
		if text == "" {
			m.err = fmt.Errorf("comment text cannot be empty")
			return nil
		}

		m.task.AddComment(text)

		// Create backup before saving
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reset add mode
		m.addMode = false
		m.textInput.Reset()
		m.err = nil

		// Move cursor to the new comment (last in list)
		m.cursor = len(m.task.Comments) - 1

		return CommentAddedMsg{Task: m.task}
	}
}

func (m *TaskCommentsModel) executeDelete() tea.Cmd {
	return func() tea.Msg {
		if !m.task.RemoveComment(m.cursor) {
			return ErrorMsg{Err: fmt.Errorf("failed to remove comment")}
		}

		// Create backup before saving
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Adjust cursor if needed
		if m.cursor >= len(m.task.Comments) && m.cursor > 0 {
			m.cursor--
		}

		return CommentDeletedMsg{Task: m.task}
	}
}

func (m *TaskCommentsModel) startEditComment() tea.Cmd {
	comment := m.task.Comments[m.cursor]
	m.editMode = true
	m.editIndex = m.cursor
	m.editTextInput.SetValue(comment.Text)
	m.editTextInput.Focus()
	return textarea.Blink
}

func (m *TaskCommentsModel) saveEditedComment() tea.Cmd {
	return func() tea.Msg {
		text := strings.TrimSpace(m.editTextInput.Value())
		if text == "" {
			m.err = fmt.Errorf("comment text cannot be empty")
			return nil
		}

		// Update the comment text (preserve timestamp)
		m.task.Comments[m.editIndex].Text = text

		// Create backup before saving
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reset edit mode
		m.editMode = false
		m.editTextInput.Reset()
		m.err = nil

		return CommentEditedMsg{Task: m.task}
	}
}

func (m *TaskCommentsModel) editInExternalEditor() tea.Cmd {
	comment := m.task.Comments[m.cursor]
	editIndex := m.cursor

	// Create temp file content
	content := fmt.Sprintf("# Comment %d on #%d: %s\n\n", editIndex, m.task.ShortID, m.task.Title)
	content += "<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n"
	content += comment.Text

	// Create temp file
	tmpFile, err := os.CreateTemp("", "tt-comment-*.md")
	if err != nil {
		m.err = fmt.Errorf("failed to create temp file: %w", err)
		return nil
	}
	tmpPath := tmpFile.Name()
	tmpFile.WriteString(content)
	tmpFile.Close()

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
			os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		// Read modified content
		modified, readErr := os.ReadFile(tmpPath)
		os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		// Parse the edited content
		newText := util.ParseEditedCommentContent(string(modified))
		if newText == "" {
			return ErrorMsg{Err: fmt.Errorf("comment text cannot be empty")}
		}

		// Check if changed
		if newText == comment.Text {
			return nil // No changes
		}

		// Update comment (preserve timestamp)
		task.Comments[editIndex].Text = newText

		// Create backup before saving
		if err := storage.BackupProject(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := storage.SaveProject(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return CommentEditedMsg{Task: task}
	})
}

func (m *TaskCommentsModel) View() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	b.WriteString(titleStyle.Render(fmt.Sprintf("Comments for #%d: %s", m.task.ShortID, m.task.Title)))
	b.WriteString("\n")

	// Comment count
	countStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	b.WriteString(countStyle.Render(fmt.Sprintf("%d comment(s)", len(m.task.Comments))))
	b.WriteString("\n\n")

	// Expanded view mode - show full comment
	if m.expandedView && len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments) {
		comment := m.task.Comments[m.cursor]
		relativeTime := storage.FormatRelativeTime(comment.CreatedAt)
		timestamp := fmt.Sprintf("[%s] %s", comment.CreatedAt.Format("2006-01-02 15:04"), relativeTime)

		b.WriteString(FieldLabelFocusedStyle.Render(fmt.Sprintf("Comment [%d]", m.cursor)))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(timestamp))
		b.WriteString("\n\n")

		// Show full comment text with word wrapping
		maxLen := m.width - 4
		if maxLen < 40 {
			maxLen = 40
		}
		lines := strings.Split(comment.Text, "\n")
		for _, line := range lines {
			// Word wrap long lines
			for len(line) > maxLen {
				b.WriteString(line[:maxLen])
				b.WriteString("\n")
				line = line[maxLen:]
			}
			b.WriteString(line)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("Enter/Esc: close expanded view"))
		return b.String()
	}

	// Edit mode input
	if m.editMode {
		comment := m.task.Comments[m.editIndex]
		relativeTime := storage.FormatRelativeTime(comment.CreatedAt)
		b.WriteString(FieldLabelFocusedStyle.Render(fmt.Sprintf("Edit Comment [%d]:", m.editIndex)))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Created: %s (%s)", comment.CreatedAt.Format("2006-01-02 15:04"), relativeTime)))
		b.WriteString("\n\n")
		b.WriteString(m.editTextInput.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Ctrl+S: save | Esc: cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// Add mode input
	if m.addMode {
		b.WriteString(FieldLabelFocusedStyle.Render("Add Comment:"))
		b.WriteString("\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Ctrl+S: save | Esc: cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// Delete confirmation
	if m.confirmDelete && len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments) {
		comment := m.task.Comments[m.cursor]
		preview := comment.Text
		if len(preview) > 50 {
			preview = preview[:47] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " ")

		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		b.WriteString(warningStyle.Render(fmt.Sprintf("Delete comment [%d]?", m.cursor)))
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  \"%s\"", preview)))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("y: confirm delete | n/Esc: cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// Error display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	// Comments list
	if len(m.task.Comments) == 0 {
		b.WriteString(MutedStyle.Render("No comments yet. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		// Calculate visible comments based on height
		headerLines := 6 // title, count, spacing, help
		availableHeight := m.height - headerLines
		maxVisible := availableHeight / 4 // ~4 lines per comment
		if maxVisible < 3 {
			maxVisible = 3
		}

		// Calculate scroll offset
		scrollOffset := 0
		if m.cursor >= maxVisible {
			scrollOffset = m.cursor - maxVisible + 1
		}

		for i := scrollOffset; i < len(m.task.Comments) && i < scrollOffset+maxVisible; i++ {
			comment := m.task.Comments[i]

			// Cursor indicator
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			// Timestamp
			relativeTime := storage.FormatRelativeTime(comment.CreatedAt)
			timestamp := fmt.Sprintf("[%s] %s", comment.CreatedAt.Format("2006-01-02 15:04"), relativeTime)

			// Build comment entry
			var entry strings.Builder
			entry.WriteString(cursor)

			// Style based on selection
			if i == m.cursor {
				entry.WriteString(BoldStyle.Render(fmt.Sprintf("[%d] ", i)))
				entry.WriteString(FieldLabelFocusedStyle.Render(timestamp))
			} else {
				entry.WriteString(MutedStyle.Render(fmt.Sprintf("[%d] ", i)))
				entry.WriteString(MutedStyle.Render(timestamp))
			}
			entry.WriteString("\n")

			// Comment text (indented, possibly truncated)
			text := comment.Text
			maxLen := m.width - 8
			if maxLen < 40 {
				maxLen = 40
			}

			// Handle multi-line comments
			lines := strings.Split(text, "\n")
			for j, line := range lines {
				if j > 1 { // Show first 2 lines
					remaining := len(lines) - 2
					entry.WriteString(fmt.Sprintf("      ... (%d more lines, press Enter to expand)\n", remaining))
					break
				}
				if len(line) > maxLen {
					line = line[:maxLen-3] + "..."
				}
				entry.WriteString("      ")
				entry.WriteString(line)
				entry.WriteString("\n")
			}

			b.WriteString(entry.String())
		}

		// Scroll indicator
		if len(m.task.Comments) > maxVisible {
			remaining := len(m.task.Comments) - scrollOffset - maxVisible
			if remaining > 0 {
				b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")

	// Help footer
	helpStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	b.WriteString(helpStyle.Render("a/n: add | e: edit | E: external editor | d/x: delete | Enter: expand | Esc: close"))

	return b.String()
}
