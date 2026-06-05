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
	return overlayPopup(base, popup, baseWidth, baseHeight, max((baseWidth-popupWidth(popup))/2, 0), true)
}

// OverlayPopupBottomLeft places a popup string on top of base content at bottom-left.
// Same as OverlayPopup but left-aligned with a 1-column margin instead of centered.
func OverlayPopupBottomLeft(base, popup string, baseWidth, baseHeight int) string {
	return overlayPopup(base, popup, baseWidth, baseHeight, 1, true)
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
// A 1-char drop shadow is added on the right and bottom edges of the popup.
func OverlayPopupDimmed(base, popup string, baseWidth, baseHeight int) string {
	return overlayPopup(DimContent(base), popup, baseWidth, baseHeight, max((baseWidth-popupWidth(popup))/2, 0), true)
}

// OverlayPopupBottomLeftDimmed dims the base content before placing the popup at bottom-left.
// A 1-char drop shadow is added on the right and bottom edges of the popup.
func OverlayPopupBottomLeftDimmed(base, popup string, baseWidth, baseHeight int) string {
	return overlayPopup(DimContent(base), popup, baseWidth, baseHeight, 1, true)
}

// OverlayPopupFlushLeftDimmed dims the base content before placing the popup at
// column 0. Use this when the popup width is already calculated to match a
// left-hand layout pane and must not shift into the adjacent pane.
func OverlayPopupFlushLeftDimmed(base, popup string, baseWidth, baseHeight int) string {
	return overlayPopup(DimContent(base), popup, baseWidth, baseHeight, 0, true)
}

func overlayPopup(base, popup string, baseWidth, baseHeight, startCol int, withShadow bool) string {
	if popup == "" {
		return base
	}

	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")
	popupHeight := len(popupLines)
	maxPopupWidth := popupWidth(popup)

	pad := ""
	if withShadow {
		pad = DimStyle.Render("")
	}
	for len(baseLines) < baseHeight {
		baseLines = append(baseLines, pad)
	}

	startRow := bottomPopupTopRow(baseHeight, popupHeight)
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

	if withShadow {
		shadowChar := ShadowStyle.Render(" ")
		// Right shadow: 1 char to the right of each popup line.
		for i := 1; i < len(popupLines); i++ {
			row := startRow + i
			if row >= len(baseLines) {
				break
			}
			shadowCol := startCol + maxPopupWidth
			if shadowCol < baseWidth {
				baseLines[row] = overlayLineAt(baseLines[row], shadowChar, shadowCol, baseWidth)
			}
		}
		// Bottom shadow: offset 1 right from the popup's left edge, 1 row below popup.
		shadowRow := startRow + popupHeight
		if shadowRow < len(baseLines) {
			shadowStart := startCol + 1
			if shadowStart < baseWidth {
				shadowWidth := min(maxPopupWidth, baseWidth-shadowStart)
				if shadowWidth > 0 {
					bottomShadow := ShadowStyle.Width(shadowWidth).Render("")
					baseLines[shadowRow] = overlayLineAt(baseLines[shadowRow], bottomShadow, shadowStart, baseWidth)
				}
			}
		}
	}

	return strings.Join(baseLines, "\n")
}

// bottomPopupTopRow returns the first rendered row for a bottom-aligned popup,
// leaving one row below it for the bottom shadow when the terminal has space.
func bottomPopupTopRow(baseHeight, popupHeight int) int {
	return max(baseHeight-popupHeight-1, 0)
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
