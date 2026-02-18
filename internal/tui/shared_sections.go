package tui

import "github.com/sglavoie/tt/internal/models"

// CustomSectionTabs returns ["All"] + sorted custom sections, or nil if no custom sections exist.
func CustomSectionTabs(project *models.Project) []string {
	if project == nil {
		return nil
	}
	custom := project.GetCustomSections()
	if len(custom) == 0 {
		return nil
	}
	tabs := make([]string, 0, len(custom)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, custom...)
	return tabs
}

// ActiveCustomSectionFilter returns the custom section name being filtered, or "" for "All".
func ActiveCustomSectionFilter(project *models.Project, tabIndex int) string {
	tabs := CustomSectionTabs(project)
	if tabs == nil || tabIndex <= 0 || tabIndex >= len(tabs) {
		return ""
	}
	return tabs[tabIndex]
}
