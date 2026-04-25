package config

import "github.com/sglavoie/tt/internal/models"

// GetGlobalSectionColor returns the color for a section from the global registry.
// Falls back to "gray" if not found.
func (c *Config) GetGlobalSectionColor(section string) string {
	if c.SectionColors == nil {
		return "gray"
	}
	if color, ok := c.SectionColors[section]; ok && color != "" {
		return color
	}
	return "gray"
}

// SetGlobalSectionColor sets a section color in the global registry.
func (c *Config) SetGlobalSectionColor(section, color string) {
	if c.SectionColors == nil {
		c.SectionColors = make(map[string]string)
	}
	c.SectionColors[section] = color
}

// MigrateProjectSectionColors adds project section colors to global registry
// if they don't already exist (first-seen wins).
func (c *Config) MigrateProjectSectionColors(projectColors map[string]string) {
	if projectColors == nil {
		return
	}
	if c.SectionColors == nil {
		c.SectionColors = make(map[string]string)
	}
	for section, color := range projectColors {
		if _, exists := c.SectionColors[section]; !exists && color != "" && color != "gray" {
			c.SectionColors[section] = color
		}
	}
}

// ApplyGlobalColorsToProject updates a project's section colors from global registry.
func (c *Config) ApplyGlobalColorsToProject(project *models.Project) {
	if project.SectionColors == nil {
		project.SectionColors = make(map[string]string)
	}
	for _, section := range project.Sections {
		if globalColor := c.GetGlobalSectionColor(section); globalColor != "gray" {
			project.SectionColors[section] = globalColor
		}
	}
}
