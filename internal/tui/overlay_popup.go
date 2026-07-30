package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// DimContent applies DimStyle to every line of content, visually receding it
// behind an overlay popup.
func DimContent(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = DimStyle.Render(line)
	}
	return strings.Join(lines, "\n")
}

// OverlayPopupDimmed dims the base content before placing the popup at
// bottom-center. Popup renderers own their complete frame and symmetric halo.
func OverlayPopupDimmed(base, popup string, baseWidth, baseHeight int) string {
	return overlayPopup(DimContent(base), popup, baseWidth, baseHeight, max((baseWidth-popupWidth(popup))/2, 0))
}

func overlayPopup(base, popup string, baseWidth, baseHeight, startCol int) string {
	if popup == "" {
		return base
	}
	popup = fitOverlayPopupHeight(popup, baseHeight)

	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")
	popupHeight := len(popupLines)
	maxPopupWidth := popupWidth(popup)
	startCol = min(max(startCol, 0), max(baseWidth-maxPopupWidth, 0))

	for len(baseLines) < baseHeight {
		baseLines = append(baseLines, "")
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

	return strings.Join(baseLines, "\n")
}

// fitOverlayPopupHeight removes only interior rows when a modal is taller than
// the viewport. The complete outer treatment remains visible: top halo, top
// border, title, help, bottom border, and bottom halo are retained whenever the
// viewport has room for them. App-level minimum-size guards keep real popup
// viewports above the four rows required for the halo and border alone.
func fitOverlayPopupHeight(popup string, maxHeight int) string {
	if popup == "" || maxHeight < 1 {
		return ""
	}
	lines := strings.Split(popup, "\n")
	if len(lines) <= maxHeight {
		return popup
	}

	headRows := min(3, (maxHeight+1)/2)
	tailRows := min(3, maxHeight-headRows)
	middleRows := max(maxHeight-headRows-tailRows, 0)

	fitted := make([]string, 0, maxHeight)
	fitted = append(fitted, lines[:headRows]...)
	middleEnd := min(headRows+middleRows, len(lines)-tailRows)
	fitted = append(fitted, lines[headRows:middleEnd]...)
	fitted = append(fitted, lines[len(lines)-tailRows:]...)
	return strings.Join(fitted, "\n")
}

// bottomPopupTopRow returns the first rendered row for a bottom-aligned popup,
// keeping the popup's own bottom halo within the available terminal height.
func bottomPopupTopRow(baseHeight, popupHeight int) int {
	return max(baseHeight-popupHeight, 0)
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
