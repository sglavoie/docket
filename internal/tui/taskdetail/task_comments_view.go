package taskdetail

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func (m *TaskCommentsModel) View() tea.View {
	if m.Task == nil {
		return tea.NewView("No task selected")
	}

	var b strings.Builder
	b.WriteString(chrome.RenderTaskScopedFullscreenTitleWithPulse(
		fmt.Sprintf("Comments · #%d: ", m.Task.ShortID),
		m.Task,
		"",
		m.CurrentWorkspacePulse(),
		m.Width(),
		m.AccentColor()))
	b.WriteString("\n\n")

	if m.ExpandedView && m.HasSelection() {
		return m.renderExpandedView(&b)
	}
	if m.EditMode {
		return m.renderEditMode(&b)
	}
	if m.AddMode {
		return m.renderAddMode(&b)
	}

	m.renderCommentsSummary(&b)
	m.renderMessages(&b)
	spec, ok := m.guidanceSpec()
	footer := renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor())
	var tail strings.Builder
	m.renderStatusMessage(&tail)
	tail.WriteString(footer)
	m.renderCommentsList(&b, taskToolAvailableRows(b.String(), tail.String(), m.Height()))
	b.WriteString(tail.String())

	return tea.NewView(b.String())
}

func (m *TaskCommentsModel) renderExpandedView(b *strings.Builder) tea.View {
	comment := m.Task.Comments[m.Cursor]
	timestamp := chrome.FormatTimestampBracket(comment.CreatedAt)
	headerLabel := fmt.Sprintf("Comment [%d]", m.Cursor)

	b.WriteString(theme.FieldLabelFocusedStyle.Render(headerLabel))
	b.WriteString("\n")
	b.WriteString(theme.MutedStyle.Render(timestamp))
	b.WriteString("\n\n")
	b.WriteString(textfit.RenderMarkdown(comment.Text, max(m.Width()-4, 1)))
	b.WriteString("\n\n")
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskCommentsModel) renderEditMode(b *strings.Builder) tea.View {
	if m.editIndex >= len(m.Task.Comments) {
		b.WriteString(theme.ValidationErrorStyle.Render("Comment was deleted externally."))
		b.WriteString("\n\n")
		spec, ok := m.guidanceSpec()
		b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
		return tea.NewView(b.String())
	}
	comment := m.Task.Comments[m.editIndex]
	b.WriteString(theme.FieldLabelFocusedStyle.Render(fmt.Sprintf("Edit Comment [%d]:", m.editIndex)))
	b.WriteString("\n")
	b.WriteString(theme.MutedStyle.Render(chrome.FormatTimestampLabeled("Created", comment.CreatedAt)))
	b.WriteString("\n\n")
	theme.ApplyThemedTextarea(&m.EditTextInput)
	b.WriteString(m.EditTextInput.View())
	b.WriteString("\n\n")
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskCommentsModel) renderAddMode(b *strings.Builder) tea.View {
	b.WriteString(theme.FieldLabelFocusedStyle.Render("Add Comment:"))
	b.WriteString("\n")
	theme.ApplyThemedTextarea(&m.TextInput)
	b.WriteString(m.TextInput.View())
	b.WriteString("\n\n")
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskCommentsModel) renderCommentsSummary(b *strings.Builder) {
	b.WriteString(theme.MutedStyle.Render(fmt.Sprintf("%d comment(s)", len(m.Task.Comments))))
	b.WriteString("\n\n")
}

func (m *TaskCommentsModel) renderMessages(b *strings.Builder) {
	if m.Err == nil {
		return
	}
	b.WriteString(theme.ValidationErrorStyle.Render(m.Err.Error()))
	b.WriteString("\n\n")
}

func (m *TaskCommentsModel) renderCommentsList(b *strings.Builder, availableRows int) {
	if len(m.Task.Comments) == 0 {
		b.WriteString(theme.MutedStyle.Render("No comments yet."))
		b.WriteString("\n")
		return
	}

	maxVisible := taskToolMaxVisible(len(m.Task.Comments), availableRows, 4)
	if maxVisible == 0 {
		return
	}
	scrollOffset := chrome.CalcScrollOffset(m.Cursor, maxVisible)
	maxLen := max(m.Width()-12, 1)

	for i := scrollOffset; i < len(m.Task.Comments) && i < scrollOffset+maxVisible; i++ {
		b.WriteString(m.renderCommentEntry(i, m.Task.Comments[i], maxLen))
	}

	if len(m.Task.Comments) > maxVisible {
		remaining := len(m.Task.Comments) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(theme.MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}

}

func (m *TaskCommentsModel) renderStatusMessage(b *strings.Builder) {
	if m.StatusMessage == "" {
		return
	}
	b.WriteString(theme.MutedStyle.Render(m.StatusMessage))
	b.WriteString("\n")
}

func (m *TaskCommentsModel) renderCommentEntry(index int, comment *models.Comment, maxLen int) string {
	var entry strings.Builder
	entry.WriteString("  ")

	timestamp := chrome.FormatTimestampBracket(comment.CreatedAt)

	if index == m.Cursor {
		entry.WriteString(theme.BoldStyle.Render(fmt.Sprintf("[%d] ", index)))
		entry.WriteString(theme.FieldLabelFocusedStyle.Render(timestamp))
	} else {
		entry.WriteString(theme.MutedStyle.Render(fmt.Sprintf("[%d] ", index)))
		entry.WriteString(theme.MutedStyle.Render(timestamp))
	}
	entry.WriteString("\n")

	renderCommentPreview(&entry, comment.Text, maxLen)

	return entry.String()
}

// renderCommentPreview shows the first two rendered lines of a comment and says
// how many remain, so a long comment costs the list a fixed three rows.
func renderCommentPreview(entry *strings.Builder, text string, maxLen int) {
	renderedLines := strings.Split(textfit.RenderMarkdown(text, maxLen), "\n")
	for j, line := range renderedLines {
		if j > 1 {
			fmt.Fprintf(entry, "      ... (%d more lines, press Enter to expand)\n", len(renderedLines)-2)
			break
		}

		entry.WriteString("      ")
		entry.WriteString(textfit.TruncateIntentional(line, maxLen, ""))
		entry.WriteString("\n")
	}
}
