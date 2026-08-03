package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// renderFullscreenTitle renders a title line for fullscreen views with optional subtitle.
// Pass an optional accent color to override the title foreground color.
func renderFullscreenTitle(title, subtitle string, accent ...color.Color) string {
	return renderFullscreenTitleContent(fullscreenTitleStyle(accent...).Render(title), subtitle)
}

func fullscreenTitleStyle(accent ...color.Color) lipgloss.Style {
	if len(accent) > 0 && accent[0] != nil {
		return lipgloss.NewStyle().Bold(true).Foreground(accent[0])
	}
	return TitleStyle
}

func renderFullscreenTitleContent(titleContent, subtitle string) string {
	t := titleContent
	if subtitle != "" {
		t += "  " + MutedStyle.Render(subtitle)
	}
	return t
}

func renderTaskScopedFullscreenTitle(prefix string, task *models.Task, subtitle string, width int, accent ...color.Color) string {
	titleStyle := fullscreenTitleStyle(accent...)
	if task == nil {
		return fitStyledChromeToWidth("fullscreen title", renderFullscreenTitleContent(titleStyle.Render(prefix), subtitle), width)
	}
	if width <= 0 {
		titleContent := titleStyle.Render(prefix) + TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(task.Title)
		return renderFullscreenTitleContent(titleContent, subtitle)
	}

	taskTitleWidth := width - lipgloss.Width(titleStyle.Render(prefix))
	if subtitle != "" {
		taskTitleWidth -= lipgloss.Width("  " + MutedStyle.Render(subtitle))
	}
	// The task title is dense data: clip it intentionally to its own budget.
	taskTitle := truncateToVisualWidth(task.Title, max(taskTitleWidth, 0))
	titleContent := titleStyle.Render(prefix) + TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(taskTitle)
	// The composed title line is chrome; once the data title is pre-fit, any
	// residual clip here means the prefix/subtitle chrome did not fit — audit it.
	return fitStyledChromeToWidth("fullscreen title", renderFullscreenTitleContent(titleContent, subtitle), width)
}

// renderPriorityBadge renders a priority indicator with color styling. An unset
// priority has no glyph of its own, so it renders nothing rather than borrowing
// the normal-priority icon.
func renderPriorityBadge(priority int) string {
	if models.ValidatePriority(priority, false) != nil {
		return ""
	}
	return GetPriorityStyle(priority).Render(PriorityIcon(priority))
}
