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
func projectBadgeColor(projectName string) lipgloss.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(projectName))
	idx := int(h.Sum32()) % len(projectColorPalette)
	return lipgloss.Color(projectColorPalette[idx])
}
