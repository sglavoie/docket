package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// renderSidebarTaskCard renders the main task info card.
func renderSidebarTaskCard(project *models.Project, task *models.Task, width int, sectionColor string) string {
	content := renderTaskHeaderContent(task, project.HasIncompleteDependencies(task), true)
	return renderSidebarCard("Task", content, width, false, sectionColor)
}

// renderSidebarDueCard renders the due date card.
// timeFormat is a Go time layout string (e.g. "15:04" or "3:04pm").
func renderSidebarDueCard(task *models.Task, width int, timeFormat string) string {
	return renderSidebarCard("Due", renderDueDateContent(task, timeFormat), width, false, "")
}

// renderSidebarMetadataCard renders tags and section in a combined card.
func renderSidebarMetadataCard(project *models.Project, task *models.Task, width int, sectionColor string) string {
	content := renderMetadataContent(task, project, width-6)
	return renderSidebarCard("Info", content, width, false, sectionColor)
}

// renderSidebarChecklistCard renders the checklist items card.
func renderSidebarChecklistCard(task *models.Task, width int) string {
	var lines []string

	completed := 0
	for _, item := range task.Checklist {
		if item.Done {
			completed++
		}
	}

	header := fmt.Sprintf("%d/%d completed", completed, len(task.Checklist))
	lines = append(lines, MutedStyle.Render(header))

	const maxVisible = 6
	for i, item := range task.Checklist {
		if i >= maxVisible {
			remaining := len(task.Checklist) - maxVisible
			lines = append(lines, MutedStyle.Render(fmt.Sprintf("  ... +%d more", remaining)))
			break
		}
		marker := "☐"
		style := NormalStyle
		if item.Done {
			marker = "☑"
			style = MutedStyle
		}
		lines = append(lines, fmt.Sprintf("  %s %s", marker, style.Render(item.Text)))
	}

	return renderSidebarCard("Checklist", strings.Join(lines, "\n"), width, false, "")
}

// renderSidebarActivityCard renders the activity timestamps card.
func renderSidebarActivityCard(task *models.Task, width int) string {
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

	return renderSidebarCard("Activity", strings.Join(lines, "\n"), width, false, "")
}

// renderSidebarCommentsCard renders the comments card showing all comments.
func renderSidebarCommentsCard(task *models.Task, width int) string {
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
		innerWidth := max(width-CardBorderOverhead, 20)
		rendered := renderMarkdown(comment.Text, innerWidth)
		if comment.Resolved {
			lines = append(lines, MutedStyle.Render(rendered))
		} else {
			lines = append(lines, rendered)
		}
	}

	return renderSidebarCard("Comments", strings.Join(lines, "\n"), width, false, "")
}

// renderSidebarAttachmentsCard renders the attachments card showing filenames only.
func renderSidebarAttachmentsCard(task *models.Task, width int) string {
	var lines []string

	maxLen := max(width-10, 20)

	for i, attachment := range task.Attachments {
		if i > 0 {
			lines = append(lines, "")
		}
		displayName := truncateToVisualWidth(attachment.Filename, maxLen)
		if attachment.IsURL {
			link := Hyperlink(attachment.Filename, displayName)
			lines = append(lines, MutedStyle.Render("🔗 ")+link)
		} else {
			lines = append(lines, MutedStyle.Render("📎 ")+displayName)
		}
	}

	return renderSidebarCard("Attachments", strings.Join(lines, "\n"), width, false, "")
}

// renderSidebarDescriptionCard renders the description card.
func renderSidebarDescriptionCard(task *models.Task, width int) string {
	innerWidth := max(width-CardBorderOverhead, 20)
	rendered := renderMarkdown(task.Description, innerWidth)
	lines := strings.Split(rendered, "\n")
	const maxLines = 5
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, MutedStyle.Render("..."))
	}

	return renderSidebarCard("Description", strings.Join(lines, "\n"), width, false, "")
}

// renderSidebarScoreCard renders the score breakdown card for the sidebar.
func renderSidebarScoreCard(task *models.Task, width int, profile *scoring.ScoringProfile) string {
	content := renderScoreCardContent(task, profile, width-6)
	return renderSidebarCard("Score", content, width, false, "")
}
