package tui

import (
	"hash/fnv"
)

// projectColorPaletteDark is a 12-color curated hex palette for dark themes:
// bright and saturated for readability on dark backgrounds.
var projectColorPaletteDark = [12]string{
	"#f7768e", "#ff9e64", "#e0af68", "#9ece6a",
	"#73daca", "#7aa2f7", "#7dcfff", "#bb9af7",
	"#9d7cd8", "#ff6b9d", "#cba050", "#2ac3de",
}

// projectColorPaletteLight is a 12-color curated hex palette for light themes:
// darker and sufficiently saturated for readability on light backgrounds.
var projectColorPaletteLight = [12]string{
	"#be123c", "#c2410c", "#a16207", "#4d7c0f",
	"#15803d", "#0369a1", "#0e7490", "#6d28d9",
	"#a21caf", "#be185d", "#b45309", "#0891b2",
}

// projectBadgeColor returns a deterministic palette color string for the given
// project name using FNV32a hash modulo 12 over the tone-appropriate palette.
//
// Call site: renderCrossProjectContent (cross_project_outliner.go) calls
// projectBadgeColor(projName) directly and builds the badge inline — it does
// not delegate to RenderCrossProjectTaskLine. row.ProjectID is populated for
// every task row during crossProjectRebuildRows() and left empty for
// IsSectionHeader and IsSeparator rows.
func projectBadgeColor(projectName string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(projectName))
	idx := int(h.Sum32()) % 12
	if activeThemeName == "light" {
		return projectColorPaletteLight[idx]
	}
	return projectColorPaletteDark[idx]
}
