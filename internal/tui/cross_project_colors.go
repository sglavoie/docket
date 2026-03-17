package tui

import (
	"hash/fnv"

	"github.com/charmbracelet/lipgloss"
)

// projectColorPalette is a 12-color ANSI 256-color palette chosen to avoid
// collision with score_badge.go (196,208,226,118,42) and shared_render.go colors.
var projectColorPalette = [12]string{
	"204", "208", "220", "118", "43", "39", "33", "99", "135", "170", "215", "87",
}

// projectBadgeColor returns a deterministic lipgloss.Color for the given
// project name using FNV32a hash modulo 12 over the palette above.
//
// Call site: a later phase's cross-project rebuildRows() must call
// projectBadgeColor(proj.Name) for each task row and pass the result as
// CrossProjectTaskLineOpts.BadgeColor to RenderCrossProjectTaskLine.
// That rebuildRows() must also set row.ProjectID = proj.ID for every task row
// and leave ProjectID empty for IsSectionHeader and IsSeparator rows
// (per ac-9b96a4a34ea5aca2e2c870290e9305f5 and ac-0d0762b06c24926f494b11a1407b68aa).
func projectBadgeColor(projectName string) lipgloss.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(projectName))
	idx := int(h.Sum32()) % len(projectColorPalette)
	return lipgloss.Color(projectColorPalette[idx])
}
