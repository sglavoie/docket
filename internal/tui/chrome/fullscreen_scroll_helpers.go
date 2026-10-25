package chrome

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// RenderScrollbar returns a viewportHeight-line string, one cell per row: a ▐
// thumb over a │ track, two glyphs so the thumb still reads where a profile
// carries no color. Returns "" when
// content fits (no scroll needed). Thumb height is proportional
// (viewportHeight²/totalItems, min 1).
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
			b.WriteString(theme.ScrollbarThumbStyle.Render("▐"))
		} else {
			b.WriteString(theme.ScrollbarTrackStyle.Render("│"))
		}
	}
	return b.String()
}

// AttachScrollbarAtWidth attaches a newline-delimited scrollbar after fitting
// each content line to contentWidth visual cells before the scrollbar. Callers
// should pass the width already reserved for content inside their layout; non-
// positive widths derive the width from the widest content line.
func AttachScrollbarAtWidth(content, scrollbar string, contentWidth int) string {
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
			cl = textfit.TruncateIntentional(cl, contentWidth, "")
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

// CalcScrollOffset returns the scroll offset needed to keep cursorIdx visible
// within a window of maxVisible items. Used by views that iterate over items
// with index-based windowing (for i := offset; i < offset+maxVisible; ...).
func CalcScrollOffset(cursorIdx, maxVisible int) int {
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

// PersistentIndexWindow adjusts *scrollOffset to keep cursorIdx visible, then
// returns the [start,end) item range for index-window renderers.
func PersistentIndexWindow(cursorIdx, totalItems, viewportHeight int, scrollOffset *int) (int, int) {
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
