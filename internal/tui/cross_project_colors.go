package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ProjectColorPair is a durable project identity color in both shipped themes.
// Entries in projectColorPalette are indexed by models.Project.ColorSlot - 1.
type ProjectColorPair struct {
	Dark  string
	Light string
}

// projectColorPalette is frozen persisted-data infrastructure. Never reorder
// entries: ColorSlot values stored in the workspace catalog address this table
// directly. Changing an existing literal requires an explicit migration.
//
// The table was curated once from the deterministic HSL/CIE76 recipe documented
// in the durable project colors plan. It is not generated at application startup.
var projectColorPalette = [config.ProjectColorSlotCount]ProjectColorPair{
	{Dark: "#e87d7d", Light: "#aa1818"},
	{Dark: "#56fb56", Light: "#0a760a"},
	{Dark: "#df56fb", Light: "#8c0da5"},
	{Dark: "#4cc7f0", Light: "#076b8d"},
	{Dark: "#f0c74c", Light: "#7e6007"},
	{Dark: "#f150ae", Light: "#9b085e"},
	{Dark: "#4c90f0", Light: "#08459b"},
	{Dark: "#4cf0c7", Light: "#066f55"},
	{Dark: "#937efc", Light: "#270da5"},
	{Dark: "#cde87d", Light: "#546b0f"},
	{Dark: "#fb9b56", Light: "#a04a0d"},
	{Dark: "#e9a5b6", Light: "#b9274c"},
	{Dark: "#a5b0e9", Light: "#2740b9"},
	{Dark: "#e87dd6", Light: "#aa1891"},
	{Dark: "#56fb9b", Light: "#097135"},
	{Dark: "#4cacf0", Light: "#085e9b"},
	{Dark: "#a5e9e9", Light: "#176d6d"},
	{Dark: "#e9c7a5", Light: "#8f571e"},
	{Dark: "#c7a5e9", Light: "#7027b9"},
	{Dark: "#f0754c", Light: "#9b2d08"},
	{Dark: "#a8fb56", Light: "#3d7109"},
	{Dark: "#e9e3a5", Light: "#6d6617"},
	{Dark: "#a5c1e9", Light: "#2764b9"},
	{Dark: "#e87daa", Light: "#aa1855"},
	{Dark: "#e9a5d2", Light: "#b52685"},
	{Dark: "#768bf4", Light: "#08219b"},
	{Dark: "#f25a80", Light: "#9b082d"},
	{Dark: "#e9a5e9", Light: "#a824a8"},
	{Dark: "#7dd6e8", Light: "#126f81"},
	{Dark: "#b271f4", Light: "#52089b"},
	{Dark: "#e9b6a5", Light: "#a84524"},
	{Dark: "#56fbb6", Light: "#097146"},
	{Dark: "#4c9ef0", Light: "#08529b"},
	{Dark: "#4cbaf0", Light: "#086a9b"},
	{Dark: "#a5e9ab", Light: "#187220"},
	{Dark: "#f04cc7", Light: "#9b0876"},
	{Dark: "#f0ac4c", Light: "#915808"},
	{Dark: "#a5a5e9", Light: "#2727b9"},
	{Dark: "#c760fb", Light: "#730da5"},
	{Dark: "#fb5671", Light: "#a50d27"},
	{Dark: "#e9d2a5", Light: "#7e5d1b"},
	{Dark: "#e3f04c", Light: "#5e6605"},
	{Dark: "#7d98e8", Light: "#183daa"},
	{Dark: "#fb8d56", Light: "#a5400d"},
	{Dark: "#f150bb", Light: "#9b086a"},
	{Dark: "#e9aba5", Light: "#b93327"},
	{Dark: "#7da1e8", Light: "#1849aa"},
	{Dark: "#f09e4c", Light: "#9b5208"},
}

// projectColor resolves the active-theme color for a project's durable slot.
// Empty theme names intentionally use the dark palette, matching theme defaults.
func projectColor(project *models.Project) color.Color {
	if project == nil || project.ColorSlot < 1 || project.ColorSlot > len(projectColorPalette) {
		return nil
	}

	pair := projectColorPalette[project.ColorSlot-1]
	if activeThemeName == "light" {
		return lipgloss.Color(pair.Light)
	}
	return lipgloss.Color(pair.Dark)
}

// projectColorStyle returns the project's foreground style, falling back to
// the existing muted style for nil, unassigned, or invalid projects.
func projectColorStyle(project *models.Project) lipgloss.Style {
	if foreground := projectColor(project); foreground != nil {
		return lipgloss.NewStyle().Foreground(foreground)
	}
	return MutedStyle
}
