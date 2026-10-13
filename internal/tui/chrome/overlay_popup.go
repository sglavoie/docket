package chrome

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/theme"
)

// DimContent applies DimStyle to every line of content, visually receding it
// behind an overlay popup.
func DimContent(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = theme.DimStyle.Render(line)
	}
	return strings.Join(lines, "\n")
}

// overlayFlankMinColumns is the narrowest side margin still worth showing the
// host through. Anything under it is a sliver of a row cut mid-word, which
// reads as debris rather than as the view the popup stands on.
const overlayFlankMinColumns = 8

// OverlayPopupDimmed dims the base content before placing the popup at
// bottom-center. Popup renderers own their complete frame and symmetric halo;
// a side margin narrower than overlayFlankMinColumns is painted with that same
// halo on the popup's rows instead of cutting the host through.
func OverlayPopupDimmed(base, popup string, baseWidth, baseHeight int) string {
	if popup == "" {
		return base
	}
	popupWidth := PopupWidth(popup)
	startCol := max((baseWidth-popupWidth)/2, 0)
	fitted := FitOverlayPopupHeight(popup, baseHeight)
	startRow := bottomPopupTopRow(baseHeight, lipgloss.Height(fitted))
	left, right := startCol, max(baseWidth-startCol-popupWidth, 0)
	if left >= overlayFlankMinColumns {
		left = 0
	}
	if right >= overlayFlankMinColumns {
		right = 0
	}
	if left > 0 || right > 0 {
		fitted = flankOverlayPopup(fitted, popupWidth, left, right)
		startCol -= left
	}
	return overlayPopupAt(DimContent(base), fitted, baseWidth, baseHeight, startRow, startCol)
}

// flankOverlayPopup widens every popup row by left and right halo cells, the
// cell renderOverlayHalo rings the frame with, so the widened popup is still
// one rectangle.
func flankOverlayPopup(popup string, popupWidth, left, right int) string {
	leftFlank := theme.OverlayHaloStyle.Render(strings.Repeat(" ", left))
	rightFlank := theme.OverlayHaloStyle.Render(strings.Repeat(" ", right))
	lines := strings.Split(popup, "\n")
	for i, line := range lines {
		if lineWidth := lipgloss.Width(line); lineWidth < popupWidth {
			line += strings.Repeat(" ", popupWidth-lineWidth)
		}
		lines[i] = leftFlank + line + rightFlank
	}
	return strings.Join(lines, "\n")
}

// OverlayPopupDimmedAt is the same compositing under an explicit anchor, for a
// popup that hangs off something the host is still drawing. The rows above
// startRow are the host's own and are never covered, so the popup is fitted to
// what is left below the anchor rather than to the whole frame; a popup taller
// than that budget loses interior rows the same way a bottom-anchored one does.
func OverlayPopupDimmedAt(base, popup string, baseWidth, baseHeight, startRow, startCol int) string {
	if popup == "" {
		return base
	}
	startRow = min(max(startRow, 0), max(baseHeight-1, 0))
	fitted := FitOverlayPopupHeight(popup, max(baseHeight-startRow, 1))
	return overlayPopupAt(DimContent(base), fitted, baseWidth, baseHeight, startRow, startCol)
}

func overlayPopupAt(base, popup string, baseWidth, baseHeight, startRow, startCol int) string {
	if popup == "" {
		return base
	}

	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")
	maxPopupWidth := PopupWidth(popup)
	startCol = min(max(startCol, 0), max(baseWidth-maxPopupWidth, 0))

	for len(baseLines) < baseHeight {
		baseLines = append(baseLines, "")
	}

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

// FitOverlayPopupHeight removes only interior rows when a modal is taller than
// the viewport. The complete outer treatment remains visible: top halo, top
// border, title, help, bottom border, and bottom halo are retained whenever the
// viewport has room for them. App-level minimum-size guards keep real popup
// viewports above the four rows required for the halo and border alone.
func FitOverlayPopupHeight(popup string, maxHeight int) string {
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

func PopupWidth(popup string) int {
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
