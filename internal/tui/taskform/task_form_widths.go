package taskform

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// The form body is a shrink-to-fit column rather than a fixed block. Every
// control names the width it prefers and the columns its row has already spent,
// and the helpers below are the one place those two meet, so a narrow terminal
// shortens a control instead of pushing its line past the frame. Above
// formBodyPreferredWidth nothing shrinks, which is what keeps a wide terminal
// rendering exactly what it rendered before the body learned to narrow.

const (
	// formLabelMarkerWidth is the "▸ " / "  " focus prefix
	// chrome.RenderFormLabelWithWidth writes ahead of every label, so a labeled
	// row has spent it before its own label column starts.
	formLabelMarkerWidth = 2

	// formTextInputChrome is what a bubbles textinput renders outside its
	// configured width — the prompt and the trailing cursor cell — measured
	// rather than guessed, so a field set to the remaining budget still lands
	// inside it.
	formTextInputChrome = 3

	// formSelectedValuePadding is the pad theme.SelectorOptionSelectedStyle puts
	// around a focused value. It is charged at every width, focused or not, so
	// moving the focus onto a row never changes where that row ends.
	formSelectedValuePadding = 2

	// formMinValueWidth is the floor every shrinking control keeps. A value
	// narrower than this says nothing at all, so the frame is allowed to lose
	// instead: below it the min-size notice is already the honest answer.
	formMinValueWidth = 8

	// formItemIndent is the four columns a Children or Extras draft row spends
	// on its focus marker before its own value starts.
	formItemIndent = 4

	// formHintGap separates a focused row's value from its key hints.
	formHintGap = 2

	// formHintSeparator is how a row's hints are joined, and therefore where
	// formFitHint is allowed to cut one off.
	formHintSeparator = " · "

	// formOverlayFrameColumns is what chrome.RenderOverlayBox spends outside the
	// inner width it is handed — a halo, a border, and a pad column on each side.
	// Every picker the form opens is sized against this one number, so a box
	// cannot claim room the frame has already spent.
	formOverlayFrameColumns = 6
)

// formOverlayInnerWidth is the inner width of a picker the form opens. preferred
// is the width the picker would take given the room, minInner the width below
// which its own title would wrap, and the terminal caps both: a box wider than
// the frame allows loses its border rather than its content.
func formOverlayInnerWidth(width, preferred, minInner int) int {
	return min(max(preferred, minInner), max(width-formOverlayFrameColumns, 1))
}

// formOverlayTitleMinInner is the inner width a rendered overlay title needs to
// sit on its own header line beside the frame's corners.
func formOverlayTitleMinInner(title string) int {
	return lipgloss.Width(title) + formOverlayFrameColumns - 2
}

// formValueWidth is the columns a labeled row has left for its value once the
// focus marker, the label column, and the focused value's own pad are paid for.
func formValueWidth(bodyWidth, labelWidth int) int {
	return max(bodyWidth-labelWidth-formLabelMarkerWidth-formSelectedValuePadding, formMinValueWidth)
}

// formInputWidth converts a column budget into the width a bubbles textinput is
// set to, capped at the width the field would rather have. The widget draws its
// prompt and cursor outside that number, which is why the budget is charged
// formTextInputChrome before the field sees it.
func formInputWidth(preferred, budget int) int {
	return max(min(preferred, budget-formTextInputChrome), formMinValueWidth)
}

// formFitHint drops whole entries from the end of a focused row's key hints
// until what is left fits the columns the value did not take, and returns "" when
// not even the first entry does. Every key a shed hint drops is still spelled in
// the tab's own guidance, so the shedding costs a reminder rather than a route.
func formFitHint(hint string, budget int) string {
	if hint == "" || budget < 1 {
		return ""
	}
	parts := strings.Split(hint, formHintSeparator)
	for n := len(parts); n > 0; n-- {
		candidate := strings.Join(parts[:n], formHintSeparator)
		if lipgloss.Width(candidate) <= budget {
			return candidate
		}
	}
	return ""
}
