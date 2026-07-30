package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

const archivedProjectMarker = "ARCHIVED"

func projectPickerDisplayText(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.Archived {
		return fmt.Sprintf("%s %s", project.Name, archivedProjectMarker)
	}
	return project.Name
}
