package taskform

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// renderExtrasTab renders the Extras tab: the collapsible Attachments and
// Comments sections.
func (m *TaskFormModel) renderExtrasTab(b *formBodyBuilder, bodyWidth int) {
	m.renderExtrasAttachmentsSection(b, bodyWidth)
	m.renderExtrasCommentsSection(b, bodyWidth)
	m.renderRelationField(b, m.fieldCountForExtras()-2, formRelationChildren, bodyWidth)
	m.renderRelationField(b, m.fieldCountForExtras()-1, formRelationDependencies, bodyWidth)
}

// renderExtrasAttachmentsSection renders the Attachments collapsible section.
// focusIndex == 0 is the attachments header; 1..N are individual drafts (when
// expanded).
func (m *TaskFormModel) renderExtrasAttachmentsSection(b *formBodyBuilder, bodyWidth int) {
	b.field(0)
	headerFocused := m.activeTab == formTabExtras && m.focusIndex == 0
	count := len(m.draft.attachments)
	expandIcon := theme.IconCollapsed
	if m.attachmentsExpanded {
		expandIcon = theme.IconExpanded
	}
	m.renderFormSectionHeader(b, expandIcon, "Attachments", count, headerFocused,
		"a toggle · Ctrl+L add", bodyWidth)

	if m.attachmentsExpanded {
		for i, att := range m.draft.attachments {
			itemFocused := m.activeTab == formTabExtras && m.focusIndex == 1+i
			b.field(1 + i)
			b.WriteString(renderAttachmentItemRow(att, itemFocused, bodyWidth))
		}
		if count == 0 {
			renderFormSectionEmptyState(b, "No attachments. Press Ctrl+L to add one.", bodyWidth)
		}
	}
}

// renderExtrasCommentsSection renders the Comments collapsible section. Its
// header sits immediately after the Attachments section, so its focus index
// depends on whether that section is expanded.
func (m *TaskFormModel) renderExtrasCommentsSection(b *formBodyBuilder, bodyWidth int) {
	headerIdx := m.extrasCommentsHeaderIdx()
	b.field(headerIdx)
	headerFocused := m.activeTab == formTabExtras && m.focusIndex == headerIdx
	count := len(m.draft.comments)
	expandIcon := theme.IconCollapsed
	if m.commentsExpanded {
		expandIcon = theme.IconExpanded
	}
	m.renderFormSectionHeader(b, expandIcon, "Comments", count, headerFocused,
		"c toggle · Ctrl+Y add", bodyWidth)

	if m.commentsExpanded {
		for i, comment := range m.draft.comments {
			itemFocused := m.activeTab == formTabExtras && m.focusIndex == headerIdx+1+i
			b.field(headerIdx + 1 + i)
			b.WriteString(renderCommentItemRow(comment, itemFocused, bodyWidth))
		}
		if count == 0 {
			renderFormSectionEmptyState(b, "No comments. Press Ctrl+Y to add one.", bodyWidth)
		}
	}
}

// renderCommentItemRow renders a single comment draft row. Comments are not
// reorderable, so the focused hint names only edit and delete.
func renderCommentItemRow(comment formCommentDraft, focused bool, bodyWidth int) string {
	line := comment.text
	if line == "" {
		line = theme.MutedStyle.Render("(empty)")
	}
	return renderFormItemRow(line, focused, "Enter edit · Ctrl+D delete", bodyWidth)
}

// renderCommentEditOverlay renders the inline comment text editor. The text is
// stored verbatim; normalizeCommentText runs in the application layer at save
// time, on the same path a captured ;comment; takes.
func (m *TaskFormModel) renderCommentEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Edit Comment")

	innerWidth := formOverlayInnerWidth(m.width, max(m.width*2/3, 40), formOverlayTitleMinInner(title))
	footerText := chrome.RenderGuidanceText(chrome.RequiredActionGuidanceSpec(
		"Enter save",
	), max(innerWidth-4, 1))

	m.commentItemInput.SetWidth(max(innerWidth-5, 1))
	theme.ApplyThemedTextInput(&m.commentItemInput)
	content := " " + m.commentItemInput.View()

	return chrome.RenderOverlayBox(content, innerWidth, title, footerText, borderStyle)
}

// renderAttachmentItemRow renders a single attachment draft row.
func renderAttachmentItemRow(att formAttachmentDraft, focused bool, bodyWidth int) string {
	value := att.value
	if value == "" {
		value = theme.MutedStyle.Render("(empty)")
	}
	return renderFormItemRow(value, focused,
		"Enter edit · Ctrl+D delete · Ctrl+J/K move", bodyWidth)
}

// renderAttachmentEditOverlay renders the inline attachment value editor. The
// value is not classified here: a URL, a path:line code reference, and a local
// file all read as one line of text until the application layer resolves them.
func (m *TaskFormModel) renderAttachmentEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimary)
	title := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true).Render("Edit Attachment")

	innerWidth := formOverlayInnerWidth(m.width, max(m.width*2/3, 40), formOverlayTitleMinInner(title))
	footerText := chrome.RenderGuidanceText(chrome.RequiredActionGuidanceSpec(
		"Enter save",
	), max(innerWidth-4, 1))

	m.attachmentItemInput.SetWidth(max(innerWidth-5, 1))
	theme.ApplyThemedTextInput(&m.attachmentItemInput)
	content := " " + m.attachmentItemInput.View()

	return chrome.RenderOverlayBox(content, innerWidth, title, footerText, borderStyle)
}
