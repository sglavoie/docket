package taskdetail

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func (m *CommentsModel) View() tea.View {
	if m.owner == nil {
		return tea.NewView("No comments to show")
	}

	// The owner splits its own title the way the task-scoped renderer needs it,
	// which is the only place the shared presentation still mentions a task: an
	// owner that is not one names itself entirely in the prefix.
	title := m.owner.Title()
	var b strings.Builder
	b.WriteString(chrome.RenderTaskScopedFullscreenTitleWithPulse(
		title.Prefix,
		title.Task,
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

func (m *CommentsModel) renderExpandedView(b *strings.Builder) tea.View {
	entry := m.entries[m.Cursor]
	timestamp := chrome.FormatTimestampBracket(entry.CreatedAt)
	headerLabel := fmt.Sprintf("Comment [%d]", m.Cursor)

	b.WriteString(theme.FieldLabelFocusedStyle.Render(headerLabel))
	b.WriteString("\n")
	b.WriteString(theme.MutedStyle.Render(timestamp))
	b.WriteString("\n\n")
	b.WriteString(textfit.RenderMarkdown(entry.Text, max(m.Width()-4, 1)))
	b.WriteString("\n\n")
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *CommentsModel) renderEditMode(b *strings.Builder) tea.View {
	if m.editIndex >= len(m.entries) {
		b.WriteString(theme.ValidationErrorStyle.Render("Comment was deleted externally."))
		b.WriteString("\n\n")
		spec, ok := m.guidanceSpec()
		b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
		return tea.NewView(b.String())
	}
	entry := m.entries[m.editIndex]
	b.WriteString(theme.FieldLabelFocusedStyle.Render(fmt.Sprintf("Edit Comment [%d]:", m.editIndex)))
	b.WriteString("\n")
	b.WriteString(theme.MutedStyle.Render(chrome.FormatTimestampLabeled("Created", entry.CreatedAt)))
	b.WriteString("\n\n")
	theme.ApplyThemedTextarea(&m.EditTextInput)
	b.WriteString(m.EditTextInput.View())
	b.WriteString("\n\n")
	m.renderMessages(b)
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *CommentsModel) renderAddMode(b *strings.Builder) tea.View {
	b.WriteString(theme.FieldLabelFocusedStyle.Render("Add Comment:"))
	b.WriteString("\n")
	theme.ApplyThemedTextarea(&m.TextInput)
	b.WriteString(m.TextInput.View())
	b.WriteString("\n\n")
	m.renderMessages(b)
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *CommentsModel) renderCommentsSummary(b *strings.Builder) {
	b.WriteString(theme.MutedStyle.Render(fmt.Sprintf("%d comment(s)", len(m.entries))))
	b.WriteString("\n\n")
}

func (m *CommentsModel) renderMessages(b *strings.Builder) {
	if m.Err == nil {
		return
	}
	b.WriteString(theme.ValidationErrorStyle.Render(m.Err.Error()))
	b.WriteString("\n\n")
}

func (m *CommentsModel) renderCommentsList(b *strings.Builder, availableRows int) {
	if len(m.entries) == 0 {
		b.WriteString(theme.MutedStyle.Render("No comments yet."))
		b.WriteString("\n")
		return
	}

	maxVisible := taskToolMaxVisible(len(m.entries), availableRows, 4)
	if maxVisible == 0 {
		return
	}
	scrollOffset := chrome.CalcScrollOffset(m.Cursor, maxVisible)
	maxLen := max(m.Width()-12, 1)

	for i := scrollOffset; i < len(m.entries) && i < scrollOffset+maxVisible; i++ {
		b.WriteString(m.renderCommentEntry(i, m.entries[i], maxLen))
	}

	if len(m.entries) > maxVisible {
		remaining := len(m.entries) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(theme.MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}

}

func (m *CommentsModel) renderStatusMessage(b *strings.Builder) {
	if m.StatusMessage == "" {
		return
	}
	b.WriteString(theme.MutedStyle.Render(m.StatusMessage))
	b.WriteString("\n")
}

func (m *CommentsModel) renderCommentEntry(index int, entry CommentEntry, maxLen int) string {
	var row strings.Builder
	row.WriteString("  ")

	timestamp := chrome.FormatTimestampBracket(entry.CreatedAt)

	if index == m.Cursor {
		row.WriteString(theme.BoldStyle.Render(fmt.Sprintf("[%d] ", index)))
		row.WriteString(theme.FieldLabelFocusedStyle.Render(timestamp))
	} else {
		row.WriteString(theme.MutedStyle.Render(fmt.Sprintf("[%d] ", index)))
		row.WriteString(theme.MutedStyle.Render(timestamp))
	}
	row.WriteString("\n")

	renderCommentPreview(&row, entry.Text, maxLen)

	return row.String()
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
