package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *TaskAttachmentsModel) View() tea.View { //nolint:gocyclo
	if m.task == nil {
		return tea.NewView("No task selected")
	}

	var b strings.Builder
	b.WriteString(renderFullscreenTitle(
		fmt.Sprintf("Attachments for #%d: %s", m.task.ShortID, m.task.Title),
		fmt.Sprintf("%d attachment(s)", len(m.task.Attachments)),
		m.AccentColor()))
	b.WriteString("\n\n")

	if m.addMode {
		return m.renderAddMode(&b)
	}

	m.renderMessages(&b)
	m.renderAttachmentsList(&b)
	m.renderHelp(&b)

	return tea.NewView(b.String())
}

func (m *TaskAttachmentsModel) renderAddMode(b *strings.Builder) tea.View {
	b.WriteString(FieldLabelFocusedStyle.Render("Add Attachment:"))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Enter a file path or URL"))
	b.WriteString("\n\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(HelpStyle.Render("Enter: add · Esc: cancel"))
	b.WriteString("\n")
	return tea.NewView(b.String())
}

func (m *TaskAttachmentsModel) renderMessages(b *strings.Builder) {
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}
	if m.statusMessage != "" {
		b.WriteString(MutedStyle.Render(m.statusMessage))
		b.WriteString("\n\n")
	}
}

func (m *TaskAttachmentsModel) renderAttachmentsList(b *strings.Builder) {
	if len(m.task.Attachments) == 0 {
		b.WriteString(MutedStyle.Render("No attachments yet. Press 'a' to add one."))
		b.WriteString("\n")
		return
	}

	headerLines := 5
	availableHeight := m.Height() - headerLines
	maxVisible := max(availableHeight/3, 3)
	scrollOffset := calcScrollOffset(m.cursor, maxVisible)

	for i := scrollOffset; i < len(m.task.Attachments) && i < scrollOffset+maxVisible; i++ {
		b.WriteString(m.renderAttachmentEntry(i, m.task.Attachments[i]))
	}

	if len(m.task.Attachments) > maxVisible {
		remaining := len(m.task.Attachments) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}
}

func (m *TaskAttachmentsModel) renderAttachmentEntry(index int, att models.Attachment) string {
	var entry strings.Builder
	entry.WriteString("  ")

	if index == m.cursor {
		entry.WriteString(BoldStyle.Render(fmt.Sprintf("[%d] ", index)))
		entry.WriteString(FieldLabelFocusedStyle.Render(m.attachmentTypeLabel(att)))
	} else {
		entry.WriteString(MutedStyle.Render(fmt.Sprintf("[%d] ", index)))
		entry.WriteString(MutedStyle.Render(m.attachmentTypeLabel(att)))
	}

	if att.IsCodeRef && !storage.CodeReferenceFileExists(att.OriginalPath) {
		entry.WriteString(" ")
		entry.WriteString(WarningIconStyle.Render("!"))
	}
	entry.WriteString("\n")

	maxLen := max(m.Width()-8, 1)
	entry.WriteString("      ")
	entry.WriteString(truncateToVisualWidth(m.attachmentDisplayName(att), maxLen))
	entry.WriteString("\n")

	if !att.IsURL && !att.IsCodeRef {
		entry.WriteString("      ")
		entry.WriteString(MutedStyle.Render("from: " + truncateToVisualWidth(att.OriginalPath, maxLen)))
		entry.WriteString("\n")
	}

	if att.IsCodeRef && !storage.CodeReferenceFileExists(att.OriginalPath) {
		entry.WriteString("      ")
		entry.WriteString(WarningIconStyle.Render("file may not exist"))
		entry.WriteString("\n")
	}

	entry.WriteString("      ")
	entry.WriteString(MutedStyle.Render(m.attachmentMeta(att)))
	entry.WriteString("\n")

	return entry.String()
}

func (m *TaskAttachmentsModel) attachmentTypeLabel(att models.Attachment) string {
	if att.IsURL {
		return "URL "
	}
	if att.IsCodeRef {
		return "CODE"
	}
	return "FILE"
}

func (m *TaskAttachmentsModel) attachmentDisplayName(att models.Attachment) string {
	if att.IsCodeRef {
		return fmt.Sprintf("%s:%d", att.OriginalPath, att.LineNumber)
	}
	return att.Filename
}

func (m *TaskAttachmentsModel) attachmentMeta(att models.Attachment) string {
	if att.IsURL || att.IsCodeRef {
		return storage.FormatRelativeTime(att.AddedAt)
	}

	size := storage.GetAttachmentSize(m.project, m.task, &att)
	sizeStr := "(missing)"
	if size > 0 {
		sizeStr = storage.FormatFileSize(size)
	}
	return fmt.Sprintf("%s | %s", sizeStr, storage.FormatRelativeTime(att.AddedAt))
}

func (m *TaskAttachmentsModel) renderHelp(b *strings.Builder) {
	isCodeRef := m.hasSelection() && m.task.Attachments[m.cursor].IsCodeRef
	helpParts := []string{"a: add", "o: open", "e: edit", "O: open dir"}
	if !isCodeRef {
		helpParts = append(helpParts, "Ctrl+O: original")
	}
	helpParts = append(helpParts, "J/K: reorder", "d: del", "u: unlink", "y: copy")
	if isCodeRef {
		helpParts = append(helpParts, "Y: copy w/line")
	}
	helpParts = append(helpParts, "?: help", "Esc: close")

	helpText := strings.Join(helpParts, " · ")
	if lipgloss.Width(helpText) > m.Width()-2 {
		mid := len(helpParts) / 2
		line1 := strings.Join(helpParts[:mid], " · ")
		line2 := strings.Join(helpParts[mid:], " · ")
		b.WriteString(renderStatusBarFooter(HelpStyle.Render(line1), "", m.Width(), m.AccentColor()))
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render(line2))
		return
	}

	b.WriteString(renderStatusBarFooter(HelpStyle.Render(helpText), "", m.Width(), m.AccentColor()))
}
