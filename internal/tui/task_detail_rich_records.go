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
		lines = append(lines, MutedStyle.Render(relative+":"))
		lines = append(lines, renderMarkdown(comment.Text, innerWidth))
	}
	return strings.Join(lines, "\n")
}

// renderDecisionsSection renders all decisions with timestamps.
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
		lines = append(lines, MutedStyle.Render(relative+":"))
		lines = append(lines, renderMarkdown(decision.Text, innerWidth))
		if strings.TrimSpace(decision.Rationale) != "" {
			lines = append(lines, MutedStyle.Render(renderMarkdown(decision.Rationale, innerWidth)))
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
		if ref, ok := gitHubIssueAttachmentRef(attachment); ok {
			label := formatGitHubIssueRef(ref)
			link := Hyperlink(ref.URL, label)
			lines = append(lines, MutedStyle.Render(IconGitHubIssue()+" ")+link)
		} else if attachment.IsURL {
			link := Hyperlink(attachment.Filename, attachment.Filename)
			lines = append(lines, MutedStyle.Render("🔗 ")+link)
		} else {
			lines = append(lines, MutedStyle.Render(IconAttachment()+" ")+attachment.Filename)
		}
	}

	return renderCard(IconAttachment()+" Attachments", strings.Join(lines, "\n"), width, DefaultCardConfig())
}

// renderLifecycleSection renders the retained task lifecycle timestamps.
func (m *TaskDetailModel) renderLifecycleSection(width int) string {
	return renderCard("Lifecycle", buildLifecycleContent(m.task), width, DefaultCardConfig())
}

// buildLifecycleContent returns the bare lifecycle body (no card border).
func buildLifecycleContent(task *models.Task) string {
	var lines []string
	if !task.CreatedAt.IsZero() {
		lines = append(lines, formatLifecycleTimestamp("Created", task.CreatedAt))
	}
	if task.CompletedAt != nil {
		lines = append(lines, formatLifecycleTimestamp("Completed", *task.CompletedAt))
	}
	if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.CreatedAt) {
		lines = append(lines, formatLifecycleTimestamp("Modified", task.UpdatedAt))
	}
	return strings.Join(lines, "\n")
}
