package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// OverlayPopup places a popup string on top of base content at bottom-center.
// Both base and popup are rendered strings. baseWidth and baseHeight define the
// dimensions of the base content area.
func OverlayPopup(base, popup string, baseWidth, baseHeight int) string {
	return overlayPopup(base, popup, baseWidth, baseHeight, max((baseWidth-popupWidth(popup))/2, 0))
}

// OverlayPopupBottomLeft places a popup string on top of base content at bottom-left.
// Same as OverlayPopup but left-aligned with a 1-column margin instead of centered.
func OverlayPopupBottomLeft(base, popup string, baseWidth, baseHeight int) string {
	return overlayPopup(base, popup, baseWidth, baseHeight, 1)
}

// DimContent applies DimStyle to every line of content, visually receding it
// behind an overlay popup.
func DimContent(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = DimStyle.Render(line)
	}
	return strings.Join(lines, "\n")
}

// OverlayPopupDimmed dims the base content before placing the popup at bottom-center.
func OverlayPopupDimmed(base, popup string, baseWidth, baseHeight int) string {
	return OverlayPopup(DimContent(base), popup, baseWidth, baseHeight)
}

// OverlayPopupBottomLeftDimmed dims the base content before placing the popup at bottom-left.
func OverlayPopupBottomLeftDimmed(base, popup string, baseWidth, baseHeight int) string {
	return OverlayPopupBottomLeft(DimContent(base), popup, baseWidth, baseHeight)
}

func overlayPopup(base, popup string, baseWidth, baseHeight, startCol int) string {
	if popup == "" {
		return base
	}

	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")
	popupHeight := len(popupLines)
	maxPopupWidth := popupWidth(popup)

	for len(baseLines) < baseHeight {
		baseLines = append(baseLines, "")
	}

	startRow := max(baseHeight-popupHeight-1, 0)
	for i, popupLine := range popupLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}

		lineWidth := lipgloss.Width(popupLine)
		if lineWidth < maxPopupWidth {
			popupLine += strings.Repeat(" ", maxPopupWidth-lineWidth)
		}
		baseLines[row] = overlayLineAt(baseLines[row], popupLine, startCol, baseWidth)
	}

	return strings.Join(baseLines, "\n")
}

func popupWidth(popup string) int {
	width := 0
	for line := range strings.SplitSeq(popup, "\n") {
		if lineWidth := lipgloss.Width(line); lineWidth > width {
			width = lineWidth
		}
	}
	return width
}

// overlayLineAt places overlayStr on top of baseLine starting at column startCol.
func overlayLineAt(baseLine, overlayStr string, startCol, totalWidth int) string {
	baseWidth := lipgloss.Width(baseLine)
	if baseWidth < totalWidth {
		baseLine += strings.Repeat(" ", totalWidth-baseWidth)
	}

	overlayWidth := lipgloss.Width(overlayStr)

	left := ""
	if startCol > 0 {
		left = ansi.Cut(baseLine, 0, startCol)
		leftWidth := lipgloss.Width(left)
		if leftWidth < startCol {
			left += strings.Repeat(" ", startCol-leftWidth)
		}
	}

	rightStart := startCol + overlayWidth
	right := ""
	if rightStart < totalWidth {
		right = ansi.Cut(baseLine, rightStart, totalWidth)
	}

	return left + overlayStr + right
}
