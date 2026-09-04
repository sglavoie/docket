package taskform

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// renderExtrasTab renders the Extras tab: the collapsible Attachments and
// Comments sections.
func (m *TaskFormModel) renderExtrasTab() string {
	var b strings.Builder

	b.WriteString(m.renderExtrasAttachmentsSection())
	b.WriteString(m.renderExtrasCommentsSection())

	return b.String()
}

// renderExtrasAttachmentsSection renders the Attachments collapsible section.
// focusIndex == 0 is the attachments header; 1..N are individual drafts (when
// expanded).
func (m *TaskFormModel) renderExtrasAttachmentsSection() string {
	var b strings.Builder

	headerFocused := m.activeTab == formTabExtras && m.focusIndex == 0
	count := len(m.draft.attachments)
	expandIcon := theme.IconCollapsed
	if m.attachmentsExpanded {
		expandIcon = theme.IconExpanded
	}
	headerLabel := fmt.Sprintf("%s Attachments (%d)", expandIcon, count)
	sep := theme.MutedStyle.Render(strings.Repeat("─", 3))

	if headerFocused {
		b.WriteString(theme.FieldLabelFocusedStyle.Render(theme.IconCursorRight + " "))
		b.WriteString(sep)
		b.WriteString(theme.BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(theme.MutedStyle.Render(strings.Repeat("─", 16)))
		b.WriteString(theme.MutedStyle.Render("  a toggle · Ctrl+L add"))
	} else {
		b.WriteString(theme.FieldLabelStyle.Render("  "))
		b.WriteString(sep)
		b.WriteString(theme.BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(theme.MutedStyle.Render(strings.Repeat("─", 16)))
	}
	b.WriteString("\n")

	if m.attachmentsExpanded {
		for i, att := range m.draft.attachments {
			itemFocused := m.activeTab == formTabExtras && m.focusIndex == 1+i
			b.WriteString(renderAttachmentItemRow(att, itemFocused))
		}
		if count == 0 {
			b.WriteString(theme.MutedStyle.Render("    No attachments. Press Ctrl+L to add one."))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderExtrasCommentsSection renders the Comments collapsible section. Its
// header sits immediately after the Attachments section, so its focus index
// depends on whether that section is expanded.
func (m *TaskFormModel) renderExtrasCommentsSection() string {
	var b strings.Builder

	headerIdx := m.extrasCommentsHeaderIdx()
	headerFocused := m.activeTab == formTabExtras && m.focusIndex == headerIdx
	count := len(m.draft.comments)
	expandIcon := theme.IconCollapsed
	if m.commentsExpanded {
		expandIcon = theme.IconExpanded
	}
	headerLabel := fmt.Sprintf("%s Comments (%d)", expandIcon, count)
	sep := theme.MutedStyle.Render(strings.Repeat("─", 3))

	if headerFocused {
		b.WriteString(theme.FieldLabelFocusedStyle.Render(theme.IconCursorRight + " "))
		b.WriteString(sep)
		b.WriteString(theme.BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(theme.MutedStyle.Render(strings.Repeat("─", 16)))
		b.WriteString(theme.MutedStyle.Render("  c toggle · Ctrl+Y add"))
	} else {
		b.WriteString(theme.FieldLabelStyle.Render("  "))
		b.WriteString(sep)
		b.WriteString(theme.BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(theme.MutedStyle.Render(strings.Repeat("─", 16)))
	}
	b.WriteString("\n")

	if m.commentsExpanded {
		for i, comment := range m.draft.comments {
			itemFocused := m.activeTab == formTabExtras && m.focusIndex == headerIdx+1+i
			b.WriteString(renderCommentItemRow(comment, itemFocused))
		}
		if count == 0 {
			b.WriteString(theme.MutedStyle.Render("    No comments. Press Ctrl+Y to add one."))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderCommentItemRow renders a single comment draft row. Comments are not
// reorderable, so the focused hint names only edit and delete.
func renderCommentItemRow(comment formCommentDraft, focused bool) string {
	line := comment.text
	if line == "" {
		line = theme.MutedStyle.Render("(empty)")
	}
	if focused {
		hint := theme.MutedStyle.Render("  Enter edit · Ctrl+D delete")
		return theme.FieldLabelFocusedStyle.Render("  "+theme.IconCursorRight+" ") + theme.SelectorOptionSelectedStyle.Render(line) + hint + "\n"
	}
	return "    " + line + "\n"
}

// renderCommentEditOverlay renders the inline comment text editor. The text is
// stored verbatim; normalizeCommentText runs in the application layer at save
// time, on the same path a captured ;comment; takes.
func (m *TaskFormModel) renderCommentEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Edit Comment")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	maxWidth := min(max(m.width*2/3, 40), m.width-4)
	innerWidth := min(max(maxWidth, titleMinInner), max(m.width-4, 1))
	footerText := chrome.RenderGuidanceText(chrome.RequiredActionGuidanceSpec(
		"Enter save",
	), max(innerWidth-4, 1))

	m.commentItemInput.SetWidth(max(innerWidth-5, 1))
	theme.ApplyThemedTextInput(&m.commentItemInput)
	content := " " + m.commentItemInput.View()

	return chrome.RenderOverlayBox(content, innerWidth, title, footerText, borderStyle)
}

// renderAttachmentItemRow renders a single attachment draft row.
func renderAttachmentItemRow(att formAttachmentDraft, focused bool) string {
	value := att.value
	if value == "" {
		value = theme.MutedStyle.Render("(empty)")
	}
	line := value
	if focused {
		hint := theme.MutedStyle.Render("  Enter edit · Ctrl+D delete · Ctrl+J/K move")
		return theme.FieldLabelFocusedStyle.Render("  "+theme.IconCursorRight+" ") + theme.SelectorOptionSelectedStyle.Render(line) + hint + "\n"
	}
	return "    " + line + "\n"
}

// renderAttachmentEditOverlay renders the inline attachment value editor. The
// value is not classified here: a URL, a path:line code reference, and a local
// file all read as one line of text until the application layer resolves them.
func (m *TaskFormModel) renderAttachmentEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Edit Attachment")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	maxWidth := min(max(m.width*2/3, 40), m.width-4)
	innerWidth := min(max(maxWidth, titleMinInner), max(m.width-4, 1))
	footerText := chrome.RenderGuidanceText(chrome.RequiredActionGuidanceSpec(
		"Enter save",
	), max(innerWidth-4, 1))

	m.attachmentItemInput.SetWidth(max(innerWidth-5, 1))
	theme.ApplyThemedTextInput(&m.attachmentItemInput)
	content := " " + m.attachmentItemInput.View()

	return chrome.RenderOverlayBox(content, innerWidth, title, footerText, borderStyle)
}
