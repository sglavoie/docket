package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

func taskMarkerStyle(marker models.TaskMarker) lipgloss.Style {
	color := ColorMuted
	switch marker {
	case models.TaskMarkerEpic:
		color = ColorPurple
	case models.TaskMarkerStory:
		color = ColorPrimary
	case models.TaskMarkerBug:
		color = ColorError
	case models.TaskMarkerIdea:
		color = ColorWarning
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true)
}

func renderTaskMarker(marker models.TaskMarker) string {
	return taskMarkerStyle(marker).Render(marker.Icon())
}

func renderTaskMarkerForTask(task *models.Task) string {
	marker, ok := task.Marker()
	if !ok {
		return ""
	}
	return renderTaskMarker(marker)
}
