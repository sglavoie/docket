package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

const archivedProjectMarker = "ARCHIVED"

func projectDiscoveryProjects(runtime *config.RuntimeState) []*models.Project {
	if runtime == nil {
		return nil
	}

	active := runtime.ActiveProjects()
	archived := runtime.ArchivedProjects()
	projects := make([]*models.Project, 0, len(active)+len(archived))
	projects = append(projects, active...)
	projects = append(projects, archived...)
	return projects
}

func projectDiscoveryDescription(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.Archived {
		return fmt.Sprintf("%s - Project: %s", archivedProjectMarker, project.Slug)
	}
	return fmt.Sprintf("Project: %s", project.Slug)
}

func projectPickerDisplayText(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.Archived {
		return fmt.Sprintf("%s %s", project.Name, archivedProjectMarker)
	}
	return project.Name
}

func projectPickerListName(project *models.Project, width int) string {
	if project == nil {
		return ""
	}
	if !project.Archived {
		return truncateToVisualWidth(project.Name, width)
	}

	markerWidth := lipgloss.Width(archivedProjectMarker) + 1
	if width <= markerWidth {
		return truncateToVisualWidth(archivedProjectMarker, width)
	}

	name := truncateToVisualWidth(project.Name, width-markerWidth)
	return fmt.Sprintf("%s %s", name, archivedProjectMarker)
}
