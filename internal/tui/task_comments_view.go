package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *TaskCommentsModel) View() tea.View {
	if m.task == nil {
		return tea.NewView("No task selected")
	}

	var b strings.Builder
	b.WriteString(renderTaskScopedFullscreenTitle(
		fmt.Sprintf("Comments · #%d: ", m.task.ShortID),
		m.task,
		"",
		m.Width(),
		m.AccentColor()))
	b.WriteString("\n\n")

	if m.expandedView && m.hasSelection() {
		return m.renderExpandedView(&b)
	}
	if m.editMode {
		return m.renderEditMode(&b)
	}
	if m.addMode {
		return m.renderAddMode(&b)
	}

	m.renderCommentsSummary(&b)
	m.renderMessages(&b)
	m.renderCommentsList(&b)
	m.renderStatusMessage(&b)
	m.renderFooter(&b)

	return tea.NewView(b.String())
}

func (m *TaskCommentsModel) renderExpandedView(b *strings.Builder) tea.View {
	comment := m.task.Comments[m.cursor]
	timestamp := FormatTimestampBracket(comment.CreatedAt)
	headerLabel := fmt.Sprintf("Comment [%d]", m.cursor)

	b.WriteString(FieldLabelFocusedStyle.Render(headerLabel))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(timestamp))
	b.WriteString("\n\n")
	b.WriteString(renderMarkdown(comment.Text, max(m.Width()-4, 1)))
	b.WriteString("\n\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter/Esc: close expanded view"), "", m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskCommentsModel) renderEditMode(b *strings.Builder) tea.View {
	if m.editIndex >= len(m.task.Comments) {
		b.WriteString(ValidationErrorStyle.Render("Comment was deleted externally. Press Esc to cancel."))
		return tea.NewView(b.String())
	}
	comment := m.task.Comments[m.editIndex]
	b.WriteString(FieldLabelFocusedStyle.Render(fmt.Sprintf("Edit Comment [%d]:", m.editIndex)))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(FormatTimestampLabeled("Created", comment.CreatedAt)))
	b.WriteString("\n\n")
	applyThemedTextarea(&m.editTextInput)
	b.WriteString(m.editTextInput.View())
	b.WriteString("\n\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"), "", m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskCommentsModel) renderAddMode(b *strings.Builder) tea.View {
	b.WriteString(FieldLabelFocusedStyle.Render("Add Comment:"))
	b.WriteString("\n")
	applyThemedTextarea(&m.textInput)
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"), "", m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskCommentsModel) renderCommentsSummary(b *strings.Builder) {
	renderTaskItemSummary(b, "comment", len(m.task.Comments))
}

func (m *TaskCommentsModel) renderMessages(b *strings.Builder) {
	renderTaskItemMessages(b, m.err)
}

func (m *TaskCommentsModel) renderCommentsList(b *strings.Builder) {
	if len(m.task.Comments) == 0 {
		b.WriteString(MutedStyle.Render("No comments yet. Press 'a' to add one."))
		b.WriteString("\n")
		return
	}

	headerLines := 6
	availableHeight := m.Height() - headerLines
	maxVisible := max(availableHeight/4, 3)
	scrollOffset := calcScrollOffset(m.cursor, maxVisible)
	maxLen := max(m.Width()-12, 1)

	for i := scrollOffset; i < len(m.task.Comments) && i < scrollOffset+maxVisible; i++ {
		b.WriteString(m.renderCommentEntry(i, m.task.Comments[i], maxLen))
	}

	if len(m.task.Comments) > maxVisible {
		remaining := len(m.task.Comments) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}

}

func (m *TaskCommentsModel) renderStatusMessage(b *strings.Builder) {
	renderTaskItemStatusMessage(b, m.statusMessage)
}

func (m *TaskCommentsModel) renderCommentEntry(index int, comment *models.Comment, maxLen int) string {
	var entry strings.Builder
	entry.WriteString("  ")

	timestamp := FormatTimestampBracket(comment.CreatedAt)

	if index == m.cursor {
		entry.WriteString(BoldStyle.Render(fmt.Sprintf("[%d] ", index)))
		entry.WriteString(FieldLabelFocusedStyle.Render(timestamp))
	} else {
		entry.WriteString(MutedStyle.Render(fmt.Sprintf("[%d] ", index)))
		entry.WriteString(MutedStyle.Render(timestamp))
	}
	entry.WriteString("\n")

	renderTaskItemPreview(&entry, comment.Text, maxLen, false)

	return entry.String()
}

func (m *TaskCommentsModel) renderFooter(b *strings.Builder) {
	b.WriteString("\n")
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine("a/n: add · e: edit · E: external editor · d/x: delete · y: copy · Y: copy all · Enter: expand · Esc: close", m.Width())))
}
