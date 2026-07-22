package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// QuickNoteOverlay is an inline overlay for quickly appending a comment to a
// task. It is stored on AppModel and rendered on top of the current view.
type QuickNoteOverlay struct {
	taskID      string
	taskTitle   string
	project     *models.Project
	coordinator SubviewCoordinator
	input       textarea.Model
}

// newQuickNoteOverlay creates and focuses a quick note overlay for the given
// task; the note commits through the subview coordinator.
func newQuickNoteOverlay(task *models.Task, project *models.Project, coordinator SubviewCoordinator) *QuickNoteOverlay {
	ti := textarea.New()
	ti.Placeholder = "Type your note..."
	ti.CharLimit = 2000
	ti.SetHeight(3)
	ti.ShowLineNumbers = false
	ti.Focus()
	applyThemedTextarea(&ti)
	return &QuickNoteOverlay{
		taskID:      task.ID,
		taskTitle:   task.Title,
		project:     project,
		coordinator: coordinator,
		input:       ti,
	}
}

// initCmd returns the command needed to start the textarea blink cursor.
func (q *QuickNoteOverlay) initCmd() tea.Cmd {
	return textarea.Blink
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg): dismiss=true means caller should clear
// the overlay; resultMsg is non-nil when a side-effect message should be dispatched.
func (q *QuickNoteOverlay) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "ctrl+s", "enter":
		text := strings.TrimSpace(q.input.Value())
		if text == "" {
			return nil, true, nil
		}
		task, err := appendComment(q.project, q.taskID, text)
		if err != nil {
			return nil, true, ErrorMsg{Err: err}
		}
		msg := commitTaskItemMutation(q.project, task, q.coordinator.AddComment(taskIdentityFor(task, q.project), text), func(fresh *models.Task) tea.Msg {
			return CommentAddedMsg{Task: fresh}
		})()
		return nil, true, msg
	case "shift+enter":
		q.input.InsertString("\n")
		return nil, false, nil
	default:
		var cmd tea.Cmd
		q.input, cmd = q.input.Update(msg)
		return cmd, false, nil
	}
}

// render returns the overlay as a bordered popup string ready for OverlayPopup.
func (q *QuickNoteOverlay) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Quick Comment")

	// Truncate task title if needed
	taskTitle := q.taskTitle
	footerText := HelpStyle.Render("Enter: save · Shift+Enter: newline · Esc: cancel")

	// Compute widths: fit within 60% of screen, at least 40 chars
	maxWidth := min(max(width*3/5, 40), width-4)

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	// Truncate task title to fit label area
	labelPrefix := "Task: "
	maxTitleLen := max(innerWidth-lipgloss.Width(labelPrefix)-2, 3)
	if len(taskTitle) > maxTitleLen {
		taskTitle = taskTitle[:maxTitleLen-1] + "…"
	}
	label := MutedStyle.Render(labelPrefix + taskTitle)

	// Size the textarea input to fit inside the box
	inputWidth := max(innerWidth-2, 10) // account for padding
	q.input.SetWidth(inputWidth)

	var content strings.Builder
	content.WriteString(label)
	content.WriteString("\n")
	applyThemedTextarea(&q.input)
	content.WriteString(q.input.View())

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}
