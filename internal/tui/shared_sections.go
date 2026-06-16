package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

// CustomSectionTabs returns ["All"] + sorted custom sections, or nil if no custom sections exist.
func CustomSectionTabs(project *models.Project) []string {
	if project == nil {
		return nil
	}
	return customSectionTabs(project.GetCustomSections())
}

// CustomSectionTabsForProjects returns ["All"] + sorted unique custom sections
// across projects, or nil if no custom sections exist.
func CustomSectionTabsForProjects(projects []*models.Project) []string {
	customSet := make(map[string]bool)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, section := range project.GetCustomSections() {
			customSet[section] = true
		}
	}
	if len(customSet) == 0 {
		return nil
	}
	custom := make([]string, 0, len(customSet))
	for section := range customSet {
		custom = append(custom, section)
	}
	sort.Strings(custom)
	return customSectionTabs(custom)
}

// CustomSectionTabColorsForProjects returns renderable section colors for the
// aggregate cross-project custom section tabs. Global colors take precedence
// when present; project colors fill in sections that have no global entry.
func CustomSectionTabColorsForProjects(projects []*models.Project, globalColors map[string]string) map[string]string {
	tabs := CustomSectionTabsForProjects(projects)
	if tabs == nil {
		return nil
	}

	colors := make(map[string]string)
	globalSections := make(map[string]bool)
	if globalColors != nil {
		for _, tab := range tabs[1:] {
			color, ok := globalColors[tab]
			if !ok || color == "" {
				continue
			}
			globalSections[tab] = true
			if isRenderableSectionTabColor(color) {
				colors[tab] = color
			}
		}
	}

	for _, project := range projects {
		if project == nil || len(project.SectionColors) == 0 {
			continue
		}
		for _, section := range project.GetCustomSections() {
			if globalSections[section] {
				continue
			}
			if _, exists := colors[section]; exists {
				continue
			}
			if color := project.SectionColors[section]; isRenderableSectionTabColor(color) {
				colors[section] = color
			}
		}
	}

	if len(colors) == 0 {
		return nil
	}
	return colors
}

// ActiveCustomSectionFilter returns the custom section name being filtered, or "" for "All".
func ActiveCustomSectionFilter(project *models.Project, tabIndex int) string {
	tabs := CustomSectionTabs(project)
	if tabs == nil || tabIndex <= 0 || tabIndex >= len(tabs) {
		return ""
	}
	return tabs[tabIndex]
}

func customSectionTabs(custom []string) []string {
	if len(custom) == 0 {
		return nil
	}
	tabs := make([]string, 0, len(custom)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, custom...)
	return tabs
}

func isRenderableSectionTabColor(color string) bool {
	return color != "" && color != "gray" && models.IsValidSectionColor(color)
}
