package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// ScrollUpIndicator returns a muted "↑ N more" string for scroll overflow above.
func ScrollUpIndicator(n int) string {
	return MutedStyle.Render(fmt.Sprintf("↑ %d more", n))
}

// ScrollDownIndicator returns a muted "↓ N more" string for scroll overflow below.
func ScrollDownIndicator(n int) string {
	return MutedStyle.Render(fmt.Sprintf("↓ %d more", n))
}

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
// delimited), pads each content line to contentWidth, and zips them together.
// If scrollbar is empty, returns content unchanged. Content lines are padded
// using lipgloss width to handle ANSI escape sequences correctly.
func AttachScrollbar(content, scrollbar string) string {
	if scrollbar == "" {
		return content
	}

	contentLines := strings.Split(content, "\n")
	scrollLines := strings.Split(scrollbar, "\n")

	// Measure max visual width across all content lines so we can pad uniformly.
	maxWidth := 0
	for _, cl := range contentLines {
		if w := lipgloss.Width(cl); w > maxWidth {
			maxWidth = w
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
		if pad := maxWidth - lipgloss.Width(cl); pad > 0 {
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

	if cursorIdx < *scrollOffset {
		*scrollOffset = cursorIdx
	}
	if cursorIdx >= *scrollOffset+contentHeight {
		*scrollOffset = cursorIdx - contentHeight + 1
	}
	maxOffset := max(len(lines)-contentHeight, 0)
	if *scrollOffset > maxOffset {
		*scrollOffset = maxOffset
	}

	visible := lines
	if *scrollOffset > 0 && *scrollOffset < len(lines) {
		visible = lines[*scrollOffset:]
	}
	if len(visible) > contentHeight {
		visible = visible[:contentHeight]
	}
	return visible
}
