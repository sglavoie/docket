package grammar

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/textgrammar"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// draftSpan is one recognized token in the draft, as byte offsets into the
// whole multiline value, and the foreground it is drawn in.
type draftSpan struct {
	start, end int
	style      lipgloss.Style
}

// draftTokenSpans parses each logical line of the draft the way capture will
// commit it — one task per line — and colors every recognized token as what it
// becomes: a tag in its chip color, a priority in its badge color, a project in
// its own color. The token capture refuses is underlined in the error color
// where it sits, read off the refusal's own column, so the draft and the
// message beside it can never disagree about which token is wrong.
func (q *QuickCaptureInput) draftTokenSpans() []draftSpan {
	value := q.Input.Value()
	var spans []draftSpan
	lineStart := 0
	for line := range strings.SplitSeq(value, "\n") {
		if strings.TrimSpace(line) != "" {
			spans = append(spans, q.lineTokenSpans(line, lineStart)...)
		}
		lineStart += len(line) + 1
	}
	return spans
}

func (q *QuickCaptureInput) lineTokenSpans(line string, offset int) []draftSpan {
	parsed := textgrammar.Parse(line, textgrammar.SurfaceCapture)
	if len(parsed.Tokens) == 0 {
		return nil
	}
	refused := -1
	if _, err := capturetext.ParseQuickCapture(line, q.Project, q.allProjects, q.skipProjectSyntax); err != nil {
		if tokenErr, ok := errors.AsType[*capturetext.CaptureTokenError](err); ok {
			refused = tokenErr.Column
		}
	}
	spans := make([]draftSpan, 0, len(parsed.Tokens))
	for _, token := range parsed.Tokens {
		if token.End <= token.Start {
			continue
		}
		if q.skipProjectSyntax && token.Field == textgrammar.FieldProject {
			continue
		}
		style := q.captureTokenStyle(token)
		if utf8.RuneCountInString(line[:token.Start]) == refused {
			style = lipgloss.NewStyle().Foreground(theme.ColorError).Underline(true)
		}
		spans = append(spans, draftSpan{start: offset + token.Start, end: offset + token.End, style: style})
	}
	return spans
}

// captureTokenStyle is the foreground a token is drawn in: the color its value
// wears once committed, where the row or Peek gives it one, else the color of
// its kind. Only foregrounds are taken, so the draft keeps the input's paper.
func (q *QuickCaptureInput) captureTokenStyle(token textgrammar.Token) lipgloss.Style {
	fg := func(style lipgloss.Style) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(style.GetForeground())
	}
	switch token.Field {
	case textgrammar.FieldPriority:
		if priority, err := strconv.Atoi(token.Value); err == nil {
			return fg(theme.GetPriorityStyle(priority))
		}
	case textgrammar.FieldTag, textgrammar.FieldContext:
		return fg(theme.TagChipStyle(token.Value))
	case textgrammar.FieldDue:
		return lipgloss.NewStyle().Foreground(theme.ColorSecondary)
	case textgrammar.FieldDefer:
		return lipgloss.NewStyle().Foreground(theme.ColorViewStandby)
	case textgrammar.FieldProject:
		if project := capturetext.ResolveProject(token.Value, q.allProjects); project != nil {
			return theme.ProjectColorStyle(project)
		}
	case textgrammar.FieldPinned:
		return fg(theme.PinnedMarkerStyle)
	case textgrammar.FieldReview:
		return fg(theme.DueTodayStyle)
	case textgrammar.FieldParent, textgrammar.FieldChild, textgrammar.FieldDependency:
		return fg(theme.RelationMarkerStyle)
	case textgrammar.FieldAttachment, textgrammar.FieldComment:
		return lipgloss.NewStyle().Foreground(theme.ColorMuted).Italic(true)
	default:
		// A destination, and any field without a committed color of its own,
		// takes the primary color so it still reads as understood.
	}
	return lipgloss.NewStyle().Foreground(theme.ColorPrimary)
}

// RenderHighlightedInput is the textarea's own view with each token's cells
// recolored. Only color changes: every character, the caret's cell, and every
// width stay exactly what the textarea drew, so the measured panel is the drawn
// one. Where the textarea's cells cannot be matched to the draft, the plain
// view stands.
func (q *QuickCaptureInput) RenderHighlightedInput() string {
	view := q.Input.View()
	spans := q.draftTokenSpans()
	if len(spans) == 0 {
		return view
	}
	cells := draftCellOffsets(q.Input)
	if cells == nil {
		return view
	}
	caretX, caretY := -1, -1
	if q.Input.Focused() {
		probe := q.Input
		probe.SetVirtualCursor(false)
		if caret := probe.Cursor(); caret != nil {
			caretX, caretY = caret.X, caret.Y
		}
	}
	lines := strings.Split(view, "\n")
	scroll := q.Input.ScrollYOffset()
	for y := range lines {
		row := y + scroll
		if row >= len(cells) {
			break
		}
		lines[y] = paintDraftRow(lines[y], cells[row], spans, caretX, caretY == y)
	}
	return strings.Join(lines, "\n")
}

// paintDraftRow recolors the runs of cells that belong to one token, leaving
// the caret's cell as the textarea drew it.
func paintDraftRow(line string, offsets []int, spans []draftSpan, caretX int, caretRow bool) string {
	spanAt := func(cell int) int {
		if cell >= len(offsets) || offsets[cell] < 0 || (caretRow && cell == caretX) {
			return -1
		}
		for i, span := range spans {
			if offsets[cell] >= span.start && offsets[cell] < span.end {
				return i
			}
		}
		return -1
	}
	width := ansi.StringWidth(line)
	var b strings.Builder
	cell := 0
	for cell < width {
		span := spanAt(cell)
		end := cell + 1
		for end < width && spanAt(end) == span {
			end++
		}
		piece := ansi.Cut(line, cell, end)
		if span >= 0 {
			piece = spans[span].style.Render(ansi.Strip(piece))
		}
		b.WriteString(piece)
		cell = end
	}
	return b.String()
}

// draftCellOffsets maps every display row of the draft, scrolled out of view
// or not, to the byte offset each of its cells shows, or -1 for a cell that
// shows none. The rows come from a fresh textarea at the same width and tall
// enough to hold the whole draft, so the wrapping is the textarea's own and no
// second layout is guessed; the offsets come from walking those rows against
// the draft in order. A cell holding a wide rune maps both of its columns.
func draftCellOffsets(input textarea.Model) [][]int {
	value := input.Value()
	probe := textarea.New()
	probe.ShowLineNumbers = false
	probe.Prompt = ""
	probe.CharLimit = 0
	probe.MaxHeight = 0
	probe.MaxWidth = 0
	probe.SetWidth(input.Width())
	probe.SetHeight(len(value) + input.LineCount() + 1)
	probe.SetValue(value)
	probe.Blur()

	rows := strings.Split(probe.View(), "\n")
	result := make([][]int, 0, len(rows))
	pos := 0
	for _, row := range rows {
		plain := ansi.Strip(row)
		if pos < len(value) && value[pos] == '\n' {
			pos++
		}
		offsets := make([]int, 0, ansi.StringWidth(plain))
		for _, r := range plain {
			w := max(ansi.StringWidth(string(r)), 1)
			mapped := -1
			for pos < len(value) && value[pos] != '\n' {
				next, size := utf8.DecodeRuneInString(value[pos:])
				if next == r || unicode.IsSpace(next) && r == ' ' {
					mapped = pos
					pos += size
					break
				}
				if !unicode.IsSpace(next) {
					break
				}
				pos += size // whitespace the textarea did not draw at a wrap
			}
			for range w {
				offsets = append(offsets, mapped)
			}
		}
		result = append(result, offsets)
	}
	if pos < len(value) {
		return nil
	}
	return result
}

// HighlightCaptureExample draws a capture line the way the composer would
// color it as typed, for surfaces that teach the grammar by example. It
// resolves no project, so a %project token takes the primary color, and it
// refuses nothing: an example is shown, never committed.
func HighlightCaptureExample(line string) string {
	parsed := textgrammar.Parse(line, textgrammar.SurfaceCapture)
	var q QuickCaptureInput
	tokens := slices.Clone(parsed.Tokens)
	slices.SortFunc(tokens, func(a, b textgrammar.Token) int { return a.Start - b.Start })
	var b strings.Builder
	cursor := 0
	for _, token := range tokens {
		if token.Start < cursor || token.End <= token.Start || token.End > len(line) {
			continue
		}
		b.WriteString(line[cursor:token.Start])
		b.WriteString(q.captureTokenStyle(token).Render(line[token.Start:token.End]))
		cursor = token.End
	}
	b.WriteString(line[cursor:])
	return b.String()
}
