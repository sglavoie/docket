package taskform

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// formBodyBuilder accumulates a tab body and records the body line each
// focusable field starts on. A field is declared once, where it starts, and its
// extent is the distance to the next declaration, so a renderer that already
// writes its rows in focus order gains bounds without bracketing anything.
//
// The bounds exist for one reason: the viewport has to know which lines the
// focused control occupies before it can scroll them into view. Nothing else
// reads them, and a tab that declares no field is simply unscrollable rather
// than broken.
type formBodyBuilder struct {
	strings.Builder
	marks []formFieldMark
}

// formFieldMark is one focusable field and the body line it begins on.
type formFieldMark struct {
	field int
	line  int
}

// field records that whatever is written next begins focusable field idx. It
// reads the line count off the bytes already written rather than tracking it
// separately, so a renderer cannot mark a field at a line it did not reach.
func (b *formBodyBuilder) field(idx int) {
	b.marks = append(b.marks, formFieldMark{field: idx, line: strings.Count(b.String(), "\n")})
}

// formFieldSpan resolves the half-open line range [start, end) field occupies.
// The end is the next declared field's start, so a field owns every line up to
// its successor — the value rows, the wrapped textarea, and the section's empty
// state alike. ok is false when the field was never declared, which is what
// leaves an overlay or an undeclared tab pinned at the top.
func formFieldSpan(marks []formFieldMark, field, totalLines int) (start, end int, ok bool) {
	for i, mark := range marks {
		if mark.field != field {
			continue
		}
		end = totalLines
		if i+1 < len(marks) {
			end = marks[i+1].line
		}
		return mark.line, max(end, mark.line+1), true
	}
	return 0, 0, false
}

// formViewportOffset is the first body line the window shows. It is a pure
// function of the body, its field bounds, the focus, and the row budget, so no
// render writes scroll state and two renders of one form cannot disagree about
// where the window sits.
//
// The focused field is scrolled into view whole when it fits, and pinned to its
// own first line when it is taller than the window: a control a reader cannot
// see the start of is worse than one whose tail is off screen, and the tail is
// one Tab away from being the next field's problem.
func formViewportOffset(totalLines int, marks []formFieldMark, focus, visibleRows int) int {
	if visibleRows < 1 || totalLines <= visibleRows {
		return 0
	}
	start, end, ok := formFieldSpan(marks, focus, totalLines)
	if !ok {
		return 0
	}
	offset := 0
	if end > visibleRows {
		offset = end - visibleRows
	}
	if start < offset {
		offset = start
	}
	return min(max(offset, 0), totalLines-visibleRows)
}

// scrollFormBody windows a form body to maxRows so the focused field stays
// visible at any height the form renders at. A body that already fits is
// returned untouched and costs nothing; a taller one spends its last row on the
// exact counts above and below, which is the one thing a reader needs to know
// that the window itself cannot show.
//
// This is what replaced the clip-and-apologize form of the same budget: the
// rows below the fold were unreachable at any focus, so the only remedy the
// notice could name was a bigger terminal.
func scrollFormBody(body string, marks []formFieldMark, focus, maxRows int) string {
	if maxRows < 1 {
		maxRows = 1
	}
	lines := strings.Split(body, "\n")
	if len(lines) <= maxRows {
		return body
	}
	visible := max(maxRows-1, 1)
	offset := formViewportOffset(len(lines), marks, focus, visible)
	end := min(offset+visible, len(lines))
	window := append([]string(nil), lines[offset:end]...)
	return strings.Join(append(window, formScrollPositionLine(offset, len(lines)-end)), "\n")
}

// formScrollPositionLine spells what sits off screen in each direction, in the
// same words Peek's position row uses, because a reader who has learned one
// window's counts has learned both.
func formScrollPositionLine(above, below int) string {
	return theme.MutedStyle.Render(fmt.Sprintf("%d above · %d below", above, below))
}

// clampFormHelpPane cuts the help column to the rows the body window occupies.
// The pane is reference text beside the thing being edited, so when the two
// disagree about height it is the pane that gives: a joined block taller than
// the budget reaches clampFormBody, which would spend the body's own last row —
// the position line — announcing that the help pane was too tall.
func clampFormHelpPane(pane string, maxRows int) string {
	if maxRows < 1 {
		maxRows = 1
	}
	lines := strings.Split(pane, "\n")
	if len(lines) <= maxRows {
		return pane
	}
	return strings.Join(append(lines[:maxRows-1:maxRows-1], theme.MutedStyle.Render("…")), "\n")
}
