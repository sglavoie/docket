package grammar

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// InlineSuggestionRemainder is the part of the selected completion the draft
// has not typed yet, drawn dimmed after the caret the way a shell suggests the
// rest of a command: `%web` with Website selected shows `site`. It answers only
// where the remainder reads as a continuation — the caret at the very end of
// the draft, at least one character typed after the trigger, and the selected
// value starting with what was typed, case aside —
// so it never suggests text that Tab would not then insert there.
func (q *QuickCaptureInput) InlineSuggestionRemainder() string {
	if !q.AC.IsActive() {
		return ""
	}
	value := q.Input.Value()
	caret := q.cursorByteOffset()
	if caret != len(value) {
		return ""
	}
	mode, _, partial := DetectTrigger(value[:caret], q.Triggers)
	// A bare trigger has nothing to continue: the dropdown already lists the
	// choices, and drawing the first one would put words in the draft's mouth.
	if mode != q.AC.Mode || partial == "" {
		return ""
	}
	selected := q.AC.Selected().Value
	if len(selected) <= len(partial) || !strings.EqualFold(selected[:len(partial)], partial) {
		return ""
	}
	remainder := selected[len(partial):]
	if strings.ContainsAny(remainder, "\n") {
		return ""
	}
	return remainder
}

// RenderInputWithInlineSuggestion is the textarea's own view with the
// remainder written into the blank cells just past the caret. The caret's cell
// comes from the textarea's own geometry, never a second layout: a copy
// reporting the real cursor names the row and column the virtual cursor is
// drawn on. Cells that are not blank, a remainder wider than what the row has
// left, or a caret the copy cannot place all fall back to the plain view
// — truncated rather than wrapped, since a suggestion is a hint and never moves
// a row.
func (q *QuickCaptureInput) RenderInputWithInlineSuggestion() string {
	view := q.Input.View()
	remainder := q.InlineSuggestionRemainder()
	if remainder == "" || !q.Input.Focused() {
		return view
	}
	probe := q.Input
	probe.SetVirtualCursor(false)
	caret := probe.Cursor()
	if caret == nil {
		return view
	}
	lines := strings.Split(view, "\n")
	y := caret.Y
	if y < 0 || y >= len(lines) {
		return view
	}
	line := lines[y]
	start := caret.X + 1 // past the virtual cursor's own cell
	width := ansi.StringWidth(line)
	available := width - start
	if available <= 0 {
		return view
	}
	ghost := textfit.FitStyledLineToWidth(remainder, available)
	ghostWidth := ansi.StringWidth(ghost)
	if ghostWidth == 0 || strings.TrimSpace(ansi.Strip(ansi.Cut(line, start, start+ghostWidth))) != "" {
		return view
	}
	style := lipgloss.NewStyle().Foreground(theme.ColorMuted).Faint(true)
	lines[y] = ansi.Cut(line, 0, start) + style.Render(ghost) + ansi.Cut(line, start+ghostWidth, width)
	return strings.Join(lines, "\n")
}
