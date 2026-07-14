package config

import "github.com/sglavoie/tt/internal/models"

// This file mirrors section_colors.go onto the Settings domain, which owns the
// global SectionColors registry, so the TUI section manager and color
// application paths can read and mutate section colors through the owning
// domain rather than the config.Config facade.

// GetGlobalSectionColor returns the color for a section from the global
// registry, falling back to "gray" if not found.
func (s *Settings) GetGlobalSectionColor(section string) string {
	if s.SectionColors == nil {
		return "gray"
	}
	if color, ok := s.SectionColors[section]; ok && color != "" {
		return color
	}
	return "gray"
}

// SetGlobalSectionColor sets a section color in the global registry.
func (s *Settings) SetGlobalSectionColor(section, color string) {
	if s.SectionColors == nil {
		s.SectionColors = make(map[string]string)
	}
	s.SectionColors[section] = color
}

// MigrateProjectSectionColors adds project section colors to the global
// registry if they don't already exist (first-seen wins).
func (s *Settings) MigrateProjectSectionColors(projectColors map[string]string) {
	if projectColors == nil {
		return
	}
	if s.SectionColors == nil {
		s.SectionColors = make(map[string]string)
	}
	for section, color := range projectColors {
		if _, exists := s.SectionColors[section]; !exists && color != "" && color != "gray" {
			s.SectionColors[section] = color
		}
	}
}

// ApplyGlobalColorsToProject updates a project's section colors from the global
// registry.
func (s *Settings) ApplyGlobalColorsToProject(project *models.Project) {
	if project.SectionColors == nil {
		project.SectionColors = make(map[string]string)
	}
	for _, section := range project.Sections {
		if globalColor := s.GetGlobalSectionColor(section); globalColor != "gray" {
			project.SectionColors[section] = globalColor
		}
	}
}
