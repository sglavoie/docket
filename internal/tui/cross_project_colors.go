package tui

import (
	"hash/fnv"
)

// projectColorPalette is a 12-color ANSI 256-color palette chosen to avoid
// collision with score_badge.go (196,208,226,118,42) and shared_render.go colors.
var projectColorPalette = [12]string{
	"204", "214", "220", "154", "43", "39", "33", "99", "135", "170", "215", "87",
}

// projectBadgeColor returns a deterministic palette color string for the given
// project name using FNV32a hash modulo 12 over the palette above.
//
// Call site: renderCrossProjectContent (cross_project_outliner.go) calls
// projectBadgeColor(projName) directly and builds the badge inline — it does
// not delegate to RenderCrossProjectTaskLine. row.ProjectID is populated for
// every task row during crossProjectRebuildRows() and left empty for
// IsSectionHeader and IsSeparator rows.
func projectBadgeColor(projectName string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(projectName))
	idx := int(h.Sum32()) % len(projectColorPalette)
	return projectColorPalette[idx]
}
