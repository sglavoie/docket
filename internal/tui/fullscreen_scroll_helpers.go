package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// RenderScrollbar returns a viewportHeight-line string of ░/█ characters
// representing the scroll position. Returns "" when content fits (no scroll
// needed). Thumb height is proportional (viewportHeight²/totalItems, min 1).
// Thumb position maps scrollOffset linearly to the track range.
func RenderScrollbar(totalItems, viewportHeight, scrollOffset int) string {
	if totalItems <= viewportHeight || viewportHeight <= 0 {
		return ""
	}

	// Thumb size: proportional, min 1
	thumbHeight := max((viewportHeight*viewportHeight)/totalItems, 1)

	// Thumb position: map scrollOffset to track range
	trackRange := viewportHeight - thumbHeight
	maxOffset := totalItems - viewportHeight
	thumbTop := 0
	if maxOffset > 0 {
		thumbTop = (scrollOffset * trackRange) / maxOffset
	}
	if thumbTop > trackRange {
		thumbTop = trackRange
	}

	var b strings.Builder
	for i := range viewportHeight {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i >= thumbTop && i < thumbTop+thumbHeight {
			b.WriteString(ScrollbarThumbStyle.Render("█"))
		} else {
			b.WriteString(ScrollbarTrackStyle.Render("░"))
		}
	}
	return b.String()
}

// AttachScrollbar takes a content block and a scrollbar string (both newline-
// delimited), pads each content line to the derived content width, and zips
// them together.
func AttachScrollbar(content, scrollbar string) string {
	return attachScrollbarAtWidth(content, scrollbar, 0)
}

// attachScrollbarAtWidth attaches a newline-delimited scrollbar after fitting
// each content line to contentWidth visual cells before the scrollbar. Callers
// should pass the width already reserved for content inside their layout; non-
// positive widths preserve AttachScrollbar's historical derived-width behavior.
func attachScrollbarAtWidth(content, scrollbar string, contentWidth int) string {
	if scrollbar == "" {
		return content
	}

	contentLines := strings.Split(content, "\n")
	scrollLines := strings.Split(scrollbar, "\n")

	if contentWidth <= 0 {
		for _, cl := range contentLines {
			if w := lipgloss.Width(cl); w > contentWidth {
				contentWidth = w
			}
		}
	}

	var b strings.Builder
	maxLines := max(len(scrollLines), len(contentLines))

	for i := range maxLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		cl := ""
		if i < len(contentLines) {
			cl = contentLines[i]
		}
		// Pad content line to uniform width so scrollbar aligns vertically.
		if lipgloss.Width(cl) > contentWidth {
			cl = truncateIntentional(cl, contentWidth, "")
		}
		if pad := contentWidth - lipgloss.Width(cl); pad > 0 {
			cl += strings.Repeat(" ", pad)
		}
		sl := ""
		if i < len(scrollLines) {
			sl = scrollLines[i]
		}
		b.WriteString(cl)
		b.WriteString(sl)
	}
	return b.String()
}

// calcScrollOffset returns the scroll offset needed to keep cursorIdx visible
// within a window of maxVisible items. Used by views that iterate over items
// with index-based windowing (for i := offset; i < offset+maxVisible; ...).
func calcScrollOffset(cursorIdx, maxVisible int) int {
	if cursorIdx >= maxVisible {
		return cursorIdx - maxVisible + 1
	}
	return 0
}

func adjustPersistentScrollOffset(cursorIdx, totalItems, viewportHeight int, scrollOffset *int) {
	if viewportHeight <= 0 {
		return
	}
	if totalItems <= 0 {
		*scrollOffset = 0
		return
	}

	cursorIdx = ClampCursor(cursorIdx, totalItems)
	if *scrollOffset < 0 {
		*scrollOffset = 0
	}
	maxOffset := max(totalItems-viewportHeight, 0)
	if *scrollOffset > maxOffset {
		*scrollOffset = maxOffset
	}

	if cursorIdx < *scrollOffset {
		*scrollOffset = cursorIdx
	}
	if cursorIdx >= *scrollOffset+viewportHeight {
		*scrollOffset = cursorIdx - viewportHeight + 1
	}
	if *scrollOffset > maxOffset {
		*scrollOffset = maxOffset
	}
}

// persistentIndexWindow adjusts *scrollOffset to keep cursorIdx visible, then
// returns the [start,end) item range for index-window renderers.
func persistentIndexWindow(cursorIdx, totalItems, viewportHeight int, scrollOffset *int) (int, int) {
	if viewportHeight <= 0 {
		return 0, 0
	}
	if totalItems <= 0 {
		*scrollOffset = 0
		return 0, 0
	}

	adjustPersistentScrollOffset(cursorIdx, totalItems, viewportHeight, scrollOffset)
	startIdx := *scrollOffset
	endIdx := min(startIdx+viewportHeight, totalItems)
	return startIdx, endIdx
}

// applyScroll returns the visible slice of lines for a non-persistent scroll.
// cursorIdx is the line index (in the lines slice) where the cursor sits;
// contentHeight is the number of lines that fit on screen.
func applyScroll(lines []string, cursorIdx, contentHeight int) []string {
	if contentHeight <= 0 {
		return nil
	}

	scrollOffset := calcScrollOffset(cursorIdx, contentHeight)

	visible := lines
	if scrollOffset > 0 && scrollOffset < len(lines) {
		visible = lines[scrollOffset:]
	}
	if len(visible) > contentHeight {
		visible = visible[:contentHeight]
	}
	return visible
}

// applyPersistentScroll adjusts *scrollOffset to keep cursorIdx visible, then
// returns the visible slice of lines. Unlike applyScroll, the scroll position
// is remembered across renders so that small cursor movements don't jump to
// cursor-anchored scrolling.
func applyPersistentScroll(lines []string, cursorIdx, contentHeight int, scrollOffset *int) []string {
	if contentHeight <= 0 {
		return nil
	}

	adjustPersistentScrollOffset(cursorIdx, len(lines), contentHeight, scrollOffset)

	visible := lines
	if *scrollOffset > 0 && *scrollOffset < len(lines) {
		visible = lines[*scrollOffset:]
	}
	if len(visible) > contentHeight {
		visible = visible[:contentHeight]
	}
	return visible
}

// persistentLineWindow adjusts *scrollOffset to keep cursorIdx visible, then
// returns the visible lines and their [start,end) indexes.
func persistentLineWindow(lines []string, cursorIdx, contentHeight int, scrollOffset *int) ([]string, int, int) {
	visible := applyPersistentScroll(lines, cursorIdx, contentHeight, scrollOffset)
	if contentHeight <= 0 {
		return nil, 0, 0
	}
	return visible, *scrollOffset, *scrollOffset + len(visible)
}
