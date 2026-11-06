package pulse

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// The day canvas's blocks, as the Outliner draws them in the room its rows
// leave: each is whole lines at a width, and each says nothing when it has
// nothing to say, so the caller decides only whether a block fits.

// dayCanvasIndent is where a canvas block's content starts, under its heading.
const dayCanvasIndent = "    "

// dayCanvasMaxDone bounds the finished titles the canvas lists before
// counting the rest.
const dayCanvasMaxDone = 5

// dayCanvasHeading is a canvas block's title: muted, under a dim rule, and
// indented past the fold column, so it never reads as a lane a key can fold.
func dayCanvasHeading(label, trailing string, width int) string {
	head := "  " + theme.MutedStyle.Bold(true).Render(label)
	if trailing != "" {
		head += theme.MutedStyle.Render("  " + trailing)
	}
	rule := width - lipgloss.Width(head) - 1
	if rule < 2 {
		return textfit.TruncateIntentional(head, width, "…")
	}
	return head + " " + lipgloss.NewStyle().Foreground(theme.ColorBorder).Render(strings.Repeat("─", rule))
}

// DoneBlock lists today's finished work, struck and muted, under a
// heading carrying the count and the week's shape. It draws nothing before
// anything is finished: the list above is the day's work, and an empty block
// would only repeat that.
func (canvas DayCanvas) DoneBlock(width, budget int) []string {
	if len(canvas.Done) == 0 || budget < 2 {
		return nil
	}
	trailing := ""
	if len(canvas.Behind) > 0 {
		trailing = "this week " + canvas.renderSparkline("") +
			theme.MutedStyle.Render(fmt.Sprintf(" %d", canvas.WeekDone()))
	}
	lines := []string{dayCanvasHeading(fmt.Sprintf("Done today · %d", len(canvas.Done)), trailing, width)}
	shown := min(len(canvas.Done), dayCanvasMaxDone, budget-1)
	if shown < len(canvas.Done) && shown == budget-1 {
		shown-- // the last row says how many more instead
	}
	shown = max(shown, 0)
	check := lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render("✓")
	struck := theme.MutedStyle.Strikethrough(true)
	for _, task := range canvas.Done[:shown] {
		slug := theme.MutedStyle.Render(" [" + task.ProjectSlug + "]")
		room := max(width-lipgloss.Width(dayCanvasIndent)-2-lipgloss.Width(slug), 4)
		lines = append(lines, dayCanvasIndent+check+" "+struck.Render(textfit.TruncateIntentional(task.Title, room, "…"))+slug)
	}
	if rest := len(canvas.Done) - shown; rest > 0 {
		lines = append(lines, dayCanvasIndent+theme.MutedStyle.Render(fmt.Sprintf("+ %d more", rest)))
	}
	return lines
}

// AheadBlock is the week ahead as one column per day: its name and
// date, then the deadlines and check-ins it brings, so a heavy day is seen
// before it arrives. Weekend names are dimmed; an empty day is a dot.
func (canvas DayCanvas) AheadBlock(width int) []string {
	if len(canvas.Ahead) == 0 {
		return nil
	}
	heading := dayCanvasHeading("Next 7 days", "", width)
	if canvas.AheadEmpty() {
		return []string{heading, dayCanvasIndent + theme.MutedStyle.Render("Nothing is dated this week.")}
	}
	cell := (width - lipgloss.Width(dayCanvasIndent)) / len(canvas.Ahead)
	if cell < 6 {
		return nil
	}
	cell = min(cell, 12)
	due := lipgloss.NewStyle().Foreground(theme.ColorSecondary)
	checkIn := lipgloss.NewStyle().Foreground(theme.ColorViewStandby)
	dim := lipgloss.NewStyle().Foreground(theme.ColorDimmed)
	var names, marks strings.Builder
	names.WriteString(dayCanvasIndent)
	marks.WriteString(dayCanvasIndent)
	for _, day := range canvas.Ahead {
		name := day.Date.Format("Mon 2")
		style := theme.MutedStyle
		if weekday := day.Date.Weekday(); weekday == time.Saturday || weekday == time.Sunday {
			style = dim
		}
		names.WriteString(padCell(style.Render(name), cell))
		var mark []string
		if day.Due > 0 {
			mark = append(mark, due.Render(fmt.Sprintf("●%d", day.Due)))
		}
		if day.CheckIns > 0 {
			mark = append(mark, checkIn.Render(fmt.Sprintf("◇%d", day.CheckIns)))
		}
		text := dim.Render("·")
		if len(mark) > 0 {
			text = strings.Join(mark, " ")
		}
		marks.WriteString(padCell(text, cell))
	}
	legend := dayCanvasIndent + due.Render("●") + theme.MutedStyle.Render(" due  ") +
		checkIn.Render("◇") + theme.MutedStyle.Render(" check-in")
	return []string{heading, names.String(), marks.String(), legend}
}

// renderSparkline draws the week behind as bars in the success ink, a day
// with nothing finished as the lowest bar dimmed, so the week keeps its seven
// columns and its shape reads even around empty days. sep goes between bars.
func (canvas DayCanvas) renderSparkline(sep string) string {
	done := lipgloss.NewStyle().Foreground(theme.ColorSuccess)
	quiet := lipgloss.NewStyle().Foreground(theme.ColorDimmed)
	var b strings.Builder
	for i, bar := range canvas.Sparkline() {
		if i > 0 {
			b.WriteString(sep)
		}
		if canvas.Behind[i].Done == 0 {
			b.WriteString(quiet.Render("▁"))
			continue
		}
		b.WriteString(done.Render(string(bar)))
	}
	return b.String()
}

// padCell pads styled text to cell columns, or cuts it to them.
func padCell(text string, cell int) string {
	if w := lipgloss.Width(text); w < cell {
		return text + strings.Repeat(" ", cell-w)
	}
	return textfit.TruncateIntentional(text, cell, "")
}

// CenterLine centers styled text in width, or returns it unpadded when it
// does not fit.
func CenterLine(text string, width int) string {
	if pad := (width - lipgloss.Width(text)) / 2; pad > 0 {
		return strings.Repeat(" ", pad) + text
	}
	return text
}

// RestingCard is the empty Today's resting card: the day's finished count
// and the week behind when anything was done, then the week ahead. It names no
// key — the explanation above it already does — and draws nothing for a scope
// with no history and nothing dated, where it would have nothing to say.
func (canvas DayCanvas) RestingCard(width int) []string {
	var lines []string
	if len(canvas.Done) > 0 {
		check := lipgloss.NewStyle().Foreground(theme.ColorSuccess).Bold(true)
		lines = append(lines,
			CenterLine(check.Render("✓")+"  "+theme.TitleStyle.Render("Today is done"), width),
			CenterLine(theme.MutedStyle.Render(fmt.Sprintf("%d done today · %d this week",
				len(canvas.Done), canvas.WeekDone())), width))
	}
	if len(canvas.Behind) > 0 && canvas.WeekDone() > 0 {
		var days strings.Builder
		for i, day := range canvas.Behind {
			if i > 0 {
				days.WriteString(" ")
			}
			days.WriteString(day.Date.Format("Mon")[:1])
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines,
			CenterLine(canvas.renderSparkline(" "), width),
			CenterLine(theme.MutedStyle.Render(days.String()), width))
	}
	if !canvas.AheadEmpty() {
		if ahead := canvas.AheadBlock(width); len(ahead) > 0 {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, ahead...)
		}
	}
	return lines
}
