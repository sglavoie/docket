package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// renderCommentsSection renders all comments with timestamps.
func (m *TaskDetailModel) renderCommentsSection(width int) string {
	return renderCard(IconComment()+" Comments", buildCommentsContent(m.task, width-CardBorderOverhead), width, DefaultCardConfig())
}

// buildCommentsContent returns the bare comments body (no card border).
// innerWidth is the visual width available for markdown rendering.
func buildCommentsContent(task *models.Task, innerWidth int) string {
	var lines []string
	for i, comment := range task.Comments {
		if i > 0 {
			lines = append(lines, "")
		}
		relative := formatRelativeTime(comment.CreatedAt)
		timestampLine := relative + ":"
		if comment.Resolved {
			timestampLine += " [resolved]"
		}
		lines = append(lines, MutedStyle.Render(timestampLine))
		rendered := renderMarkdown(comment.Text, innerWidth)
		if comment.Resolved {
			lines = append(lines, MutedStyle.Render(rendered))
		} else {
			lines = append(lines, rendered)
		}
	}
	return strings.Join(lines, "\n")
}

// renderDecisionsSection renders all decisions with category and timestamps.
func (m *TaskDetailModel) renderDecisionsSection(width int) string {
	return renderCard(IconDecision()+" Decisions", buildDecisionsContent(m.task, width-CardBorderOverhead), width, DefaultCardConfig())
}

// buildDecisionsContent returns the bare decisions body (no card border).
// innerWidth is the visual width available for markdown rendering.
func buildDecisionsContent(task *models.Task, innerWidth int) string {
	var lines []string
	for i, decision := range task.Decisions {
		if i > 0 {
			lines = append(lines, "")
		}
		relative := formatRelativeTime(decision.CreatedAt)
		header := relative + " [" + string(decision.Category) + "]:"
		if decision.Resolved {
			header += " [resolved]"
		}
		lines = append(lines, MutedStyle.Render(header))
		rendered := renderMarkdown(decision.Text, innerWidth)
		if decision.Resolved {
			lines = append(lines, MutedStyle.Render(rendered))
		} else {
			lines = append(lines, rendered)
		}
	}
	return strings.Join(lines, "\n")
}

// renderAttachmentsSection renders all attachments.
func (m *TaskDetailModel) renderAttachmentsSection(width int) string {
	var lines []string

	for i, attachment := range m.task.Attachments {
		if i > 0 {
			lines = append(lines, "")
		}
		if attachment.IsURL {
			link := Hyperlink(attachment.Filename, attachment.Filename)
			lines = append(lines, MutedStyle.Render("🔗 ")+link)
		} else {
			lines = append(lines, MutedStyle.Render(IconAttachment()+" ")+attachment.Filename)
		}
	}

	return renderCard(IconAttachment()+" Attachments", strings.Join(lines, "\n"), width, DefaultCardConfig())
}

// renderActivitySection renders activity timestamps.
func (m *TaskDetailModel) renderActivitySection(width int) string {
	return renderCard("Activity", buildActivityContent(m.task), width, DefaultCardConfig())
}

// buildActivityContent returns the bare activity body (no card border).
func buildActivityContent(task *models.Task) string {
	var lines []string
	if !task.CreatedAt.IsZero() {
		lines = append(lines, formatActivityEntry("Created", task.CreatedAt))
	}
	if task.CompletedAt != nil {
		lines = append(lines, formatActivityEntry("Completed", *task.CompletedAt))
	}
	if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.CreatedAt) {
		lines = append(lines, formatActivityEntry("Modified", task.UpdatedAt))
	}
	return strings.Join(lines, "\n")
}
