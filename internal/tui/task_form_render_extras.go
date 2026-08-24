package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
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
	expandIcon := IconCollapsed
	if m.attachmentsExpanded {
		expandIcon = IconExpanded
	}
	headerLabel := fmt.Sprintf("%s Attachments (%d)", expandIcon, count)
	sep := MutedStyle.Render(strings.Repeat("─", 3))

	if headerFocused {
		b.WriteString(FieldLabelFocusedStyle.Render(IconCursorRight + " "))
		b.WriteString(sep)
		b.WriteString(BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(MutedStyle.Render(strings.Repeat("─", 16)))
		b.WriteString(MutedStyle.Render("  a toggle · Ctrl+L add"))
	} else {
		b.WriteString(FieldLabelStyle.Render("  "))
		b.WriteString(sep)
		b.WriteString(BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(MutedStyle.Render(strings.Repeat("─", 16)))
	}
	b.WriteString("\n")

	if m.attachmentsExpanded {
		for i, att := range m.draft.attachments {
			itemFocused := m.activeTab == formTabExtras && m.focusIndex == 1+i
			b.WriteString(renderAttachmentItemRow(att, itemFocused))
		}
		if count == 0 {
			b.WriteString(MutedStyle.Render("    No attachments. Press Ctrl+L to add one."))
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
	expandIcon := IconCollapsed
	if m.commentsExpanded {
		expandIcon = IconExpanded
	}
	headerLabel := fmt.Sprintf("%s Comments (%d)", expandIcon, count)
	sep := MutedStyle.Render(strings.Repeat("─", 3))

	if headerFocused {
		b.WriteString(FieldLabelFocusedStyle.Render(IconCursorRight + " "))
		b.WriteString(sep)
		b.WriteString(BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(MutedStyle.Render(strings.Repeat("─", 16)))
		b.WriteString(MutedStyle.Render("  c toggle · Ctrl+Y add"))
	} else {
		b.WriteString(FieldLabelStyle.Render("  "))
		b.WriteString(sep)
		b.WriteString(BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(MutedStyle.Render(strings.Repeat("─", 16)))
	}
	b.WriteString("\n")

	if m.commentsExpanded {
		for i, comment := range m.draft.comments {
			itemFocused := m.activeTab == formTabExtras && m.focusIndex == headerIdx+1+i
			b.WriteString(renderCommentItemRow(comment, itemFocused))
		}
		if count == 0 {
			b.WriteString(MutedStyle.Render("    No comments. Press Ctrl+Y to add one."))
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
		line = MutedStyle.Render("(empty)")
	}
	if focused {
		hint := MutedStyle.Render("  Enter edit · Ctrl+D delete")
		return FieldLabelFocusedStyle.Render("  "+IconCursorRight+" ") + SelectorOptionSelectedStyle.Render(line) + hint + "\n"
	}
	return "    " + line + "\n"
}

// renderCommentEditOverlay renders the inline comment text editor. The text is
// stored verbatim; normalizeCommentText runs in the application layer at save
// time, on the same path a captured ;comment; takes.
func (m *TaskFormModel) renderCommentEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Edit Comment")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	maxWidth := min(max(m.width*2/3, 40), m.width-4)
	innerWidth := min(max(maxWidth, titleMinInner), max(m.width-4, 1))
	footerText := renderGuidanceText(requiredActionGuidanceSpec(
		"Enter save",
	), max(innerWidth-4, 1))

	m.commentItemInput.SetWidth(max(innerWidth-5, 1))
	applyThemedTextInput(&m.commentItemInput)
	content := " " + m.commentItemInput.View()

	return renderOverlayBox(content, innerWidth, title, footerText, borderStyle)
}

// renderAttachmentItemRow renders a single attachment draft row.
func renderAttachmentItemRow(att formAttachmentDraft, focused bool) string {
	value := att.value
	if value == "" {
		value = MutedStyle.Render("(empty)")
	}
	line := value
	if focused {
		hint := MutedStyle.Render("  Enter edit · Ctrl+D delete · Ctrl+J/K move")
		return FieldLabelFocusedStyle.Render("  "+IconCursorRight+" ") + SelectorOptionSelectedStyle.Render(line) + hint + "\n"
	}
	return "    " + line + "\n"
}

// renderAttachmentEditOverlay renders the inline attachment value editor. The
// value is not classified here: a URL, a path:line code reference, and a local
// file all read as one line of text until the application layer resolves them.
func (m *TaskFormModel) renderAttachmentEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Edit Attachment")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	maxWidth := min(max(m.width*2/3, 40), m.width-4)
	innerWidth := min(max(maxWidth, titleMinInner), max(m.width-4, 1))
	footerText := renderGuidanceText(requiredActionGuidanceSpec(
		"Enter save",
	), max(innerWidth-4, 1))

	m.attachmentItemInput.SetWidth(max(innerWidth-5, 1))
	applyThemedTextInput(&m.attachmentItemInput)
	content := " " + m.attachmentItemInput.View()

	return renderOverlayBox(content, innerWidth, title, footerText, borderStyle)
}
