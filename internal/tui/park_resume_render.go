package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const parkedResumeIcon = "⏸"

const (
	parkedRowSnippetDefaultWidth = 24
	parkedRowSnippetMinWidth     = 8
	parkedRowSnippetMaxWidth     = 48
)

func parkedResumeNote(task *models.Task) string {
	if task == nil {
		return "parked"
	}
	if note := strings.TrimSpace(task.ParkedNote); note != "" {
		return note
	}
	return "parked"
}

func parkedResumeRelative(task *models.Task) string {
	if task != nil && task.ParkedAt != nil {
		return formatRelativeTime(*task.ParkedAt)
	}
	return "parked"
}

func parkedResumeSnippet(task *models.Task, width int) string {
	prefix := parkedResumeIcon + " "
	if width <= lipgloss.Width(prefix) {
		return truncateToVisualWidth(parkedResumeIcon, width)
	}
	return prefix + truncateToVisualWidth(parkedResumeNote(task), width-lipgloss.Width(prefix))
}

func parkedResumeSnippetForRowWidth(task *models.Task, rowWidth int) string {
	if task == nil || !task.IsParked() {
		return ""
	}
	return parkedResumeSnippet(task, parkedRowSnippetRenderWidth(rowWidth))
}

func parkedRowSnippet(task *models.Task, rowWidth int) string {
	snippet := parkedResumeSnippetForRowWidth(task, rowWidth)
	if snippet == "" {
		return ""
	}
	return MutedStyle.Render(snippet)
}

func appendParkedRowSnippet(suffix string, task *models.Task, rowWidth int) string {
	parked := parkedRowSnippet(task, rowWidth)
	if parked == "" {
		return suffix
	}
	if suffix == "" {
		return " " + parked
	}
	return suffix + " " + parked
}

func parkedRowSnippetRenderWidth(rowWidth int) int {
	if rowWidth <= 0 {
		return parkedRowSnippetDefaultWidth
	}
	return min(max(rowWidth/3, parkedRowSnippetMinWidth), min(rowWidth, parkedRowSnippetMaxWidth))
}
