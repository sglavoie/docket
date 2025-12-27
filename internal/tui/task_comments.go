package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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

	return &TaskCommentsModel{
		task:      task,
		project:   project,
		cursor:    0,
		addMode:   false,
		textInput: ti,
	}
}

func (m *TaskCommentsModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskCommentsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.textInput.SetWidth(width - 4)
}

func (m *TaskCommentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle add mode differently
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

		case "d", "x":
			// Delete selected comment
			if len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments) {
				return m, m.deleteComment()
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

func (m *TaskCommentsModel) deleteComment() tea.Cmd {
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

		return nil
	}
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
					entry.WriteString(fmt.Sprintf("      ... (%d more lines)\n", remaining))
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
	b.WriteString(helpStyle.Render("a/n: add | d/x: delete | j/k: navigate | g/G: top/bottom | Esc: close"))

	return b.String()
}
