package taskdetail

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func (m *TaskAttachmentsModel) View() tea.View { //nolint:gocyclo
	if m.Task == nil {
		return tea.NewView("No task selected")
	}

	var b strings.Builder
	b.WriteString(chrome.RenderTaskScopedFullscreenTitleWithPulse(
		fmt.Sprintf("Attachments for #%d: ", m.Task.ShortID),
		m.Task,
		fmt.Sprintf("%d attachment(s)", len(m.Task.Attachments)),
		m.CurrentWorkspacePulse(),
		m.Width(),
		m.AccentColor()))
	b.WriteString("\n\n")

	if m.AddMode {
		return m.renderAddMode(&b)
	}

	m.renderMessages(&b)
	spec, ok := m.guidanceSpec()
	footer := renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor())
	m.renderAttachmentsList(&b, taskToolAvailableRows(b.String(), footer, m.Height()))
	b.WriteString(footer)

	return tea.NewView(b.String())
}

func (m *TaskAttachmentsModel) renderAddMode(b *strings.Builder) tea.View {
	b.WriteString(theme.FieldLabelFocusedStyle.Render("Add Attachment:"))
	b.WriteString("\n")
	b.WriteString(theme.MutedStyle.Render("Enter a file path or URL"))
	b.WriteString("\n\n")
	b.WriteString(m.TextInput.View())
	b.WriteString("\n\n")
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskAttachmentsModel) renderMessages(b *strings.Builder) {
	if m.Err != nil {
		b.WriteString(theme.ValidationErrorStyle.Render(m.Err.Error()))
		b.WriteString("\n\n")
	}
	if m.StatusMessage != "" {
		b.WriteString(theme.MutedStyle.Render(m.StatusMessage))
		b.WriteString("\n\n")
	}
}

func (m *TaskAttachmentsModel) renderAttachmentsList(b *strings.Builder, availableRows int) {
	if len(m.Task.Attachments) == 0 {
		b.WriteString(theme.MutedStyle.Render("No attachments yet."))
		b.WriteString("\n")
		return
	}

	maxVisible := taskToolMaxVisible(len(m.Task.Attachments), availableRows, 4)
	if maxVisible == 0 {
		return
	}
	scrollOffset := chrome.CalcScrollOffset(m.Cursor, maxVisible)

	for i := scrollOffset; i < len(m.Task.Attachments) && i < scrollOffset+maxVisible; i++ {
		b.WriteString(m.renderAttachmentEntry(i, m.Task.Attachments[i]))
	}

	if len(m.Task.Attachments) > maxVisible {
		remaining := len(m.Task.Attachments) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(theme.MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}
}

func (m *TaskAttachmentsModel) renderAttachmentEntry(index int, att models.Attachment) string {
	var entry strings.Builder
	entry.WriteString("  ")

	if index == m.Cursor {
		entry.WriteString(theme.BoldStyle.Render(fmt.Sprintf("[%d] ", index)))
		entry.WriteString(theme.FieldLabelFocusedStyle.Render(m.attachmentTypeLabel(att)))
	} else {
		entry.WriteString(theme.MutedStyle.Render(fmt.Sprintf("[%d] ", index)))
		entry.WriteString(theme.MutedStyle.Render(m.attachmentTypeLabel(att)))
	}

	if att.IsCodeRef && !storage.CodeReferenceFileExists(att.OriginalPath) {
		entry.WriteString(" ")
		entry.WriteString(theme.WarningIconStyle.Render("!"))
	}
	entry.WriteString("\n")

	maxLen := max(m.Width()-8, 1)
	entry.WriteString("      ")
	entry.WriteString(textfit.TruncateToVisualWidth(m.attachmentDisplayName(att), maxLen))
	entry.WriteString("\n")

	if !att.IsURL && !att.IsCodeRef {
		entry.WriteString("      ")
		entry.WriteString(theme.MutedStyle.Render("from: " + textfit.TruncateToVisualWidth(att.OriginalPath, maxLen)))
		entry.WriteString("\n")
	}

	if att.IsCodeRef && !storage.CodeReferenceFileExists(att.OriginalPath) {
		entry.WriteString("      ")
		entry.WriteString(theme.WarningIconStyle.Render("file may not exist"))
		entry.WriteString("\n")
	}

	entry.WriteString("      ")
	entry.WriteString(theme.MutedStyle.Render(m.attachmentMeta(att)))
	entry.WriteString("\n")

	return entry.String()
}

func (m *TaskAttachmentsModel) attachmentTypeLabel(att models.Attachment) string {
	if _, ok := listsurface.GitHubIssueAttachmentRef(att); ok {
		return "GITHUB ISSUE"
	}
	if att.IsURL {
		return "URL "
	}
	if att.IsCodeRef {
		return "CODE"
	}
	return "FILE"
}

func (m *TaskAttachmentsModel) attachmentDisplayName(att models.Attachment) string {
	return listsurface.AttachmentDisplayName(att)
}

func (m *TaskAttachmentsModel) attachmentMeta(att models.Attachment) string {
	if att.IsURL || att.IsCodeRef {
		return storage.FormatRelativeTime(att.AddedAt)
	}

	size := storage.GetAttachmentSize(m.project, m.Task, &att)
	sizeStr := "(missing)"
	if size > 0 {
		sizeStr = storage.FormatFileSize(size)
	}
	return fmt.Sprintf("%s | %s", sizeStr, storage.FormatRelativeTime(att.AddedAt))
}
