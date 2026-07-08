package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

const parkedResumeIcon = "⏸"

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
