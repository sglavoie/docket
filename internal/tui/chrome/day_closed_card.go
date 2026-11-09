package chrome

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// DayClosedDone is one task the day finished, as the closing card lists it.
type DayClosedDone struct {
	Title       string
	ProjectSlug string
}

// DayClosedCardSpec is the card a Close the Day pass ends on: what the day
// finished, where the pass sent the rest, and the week behind. Every value is
// read before rendering; Week arrives styled, since its bars are the canvas's.
type DayClosedCardSpec struct {
	Date    string
	Done    []DayClosedDone
	Next    int
	Backlog int
	// StillToday is the open work Today holds once the pass is over, which is
	// what decides whether the day closes clear.
	StillToday int
	Week       string
	// History is the completion history the activity grid draws, keyed by
	// local day, and Now the moment it is read against. A nil History draws no
	// grid, which is how a caller short of rows leaves it out.
	History map[time.Time]int
	Now     time.Time
}

// dayClosedMaxDone bounds the finished titles listed before counting the rest,
// the same five the day canvas lists.
const dayClosedMaxDone = 5

// dayClosedGridWeeks is how many weeks of history the closing card's activity
// grid reaches back, narrowed further by the card's width.
const dayClosedGridWeeks = 26

// RenderDayClosedCard draws the closing card. Like the triage card it names no
// keys: the footer under it does.
func RenderDayClosedCard(spec DayClosedCardSpec, width int) string {
	innerWidth := TriageCardInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)

	label := lipgloss.NewStyle().Foreground(theme.ColorSuccess).Bold(true).Render("Day closed")
	date := theme.MutedStyle.Render(spec.Date)
	header := label
	if gap := contentWidth - lipgloss.Width(label) - lipgloss.Width(date); spec.Date != "" && gap >= 1 {
		header += strings.Repeat(" ", gap) + date
	}

	lines := []string{triageProgressBar(1, 1, contentWidth), ""}
	check := lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render("✓")
	if len(spec.Done) == 0 {
		lines = append(lines, theme.MutedStyle.Render("Nothing finished today."))
	} else {
		noun := "tasks"
		if len(spec.Done) == 1 {
			noun = "task"
		}
		lines = append(lines, check+" "+theme.BoldPrimaryStyle.Render(fmt.Sprintf("%d %s done today", len(spec.Done), noun)))
		struck := theme.MutedStyle.Strikethrough(true)
		shown := min(len(spec.Done), dayClosedMaxDone)
		for _, task := range spec.Done[:shown] {
			slug := ""
			if task.ProjectSlug != "" {
				slug = theme.MutedStyle.Render(" " + theme.ProjectToken(task.ProjectSlug))
			}
			room := max(contentWidth-2-lipgloss.Width(slug), 4)
			lines = append(lines, "  "+struck.Render(textfit.TruncateIntentional(task.Title, room, "…"))+slug)
		}
		if rest := len(spec.Done) - shown; rest > 0 {
			lines = append(lines, theme.MutedStyle.Render(fmt.Sprintf("  + %d more", rest)))
		}
	}

	if settled := dayClosedSettled(spec); settled != "" {
		lines = append(lines, "", theme.MutedStyle.Render(textfit.TruncateIntentional(settled, contentWidth, "…")))
	}
	if spec.Week != "" {
		lines = append(lines, theme.MutedStyle.Render("This week ")+spec.Week)
	}
	if grid := pulse.ActivityGrid(spec.History, spec.Now, dayClosedGridWeeks, contentWidth); len(grid) > 0 {
		lines = append(lines, "")
		lines = append(lines, grid...)
		lines = append(lines, pulse.ActivityLegendLines(spec.History, spec.Now, pulse.ActivityGridWeeks(dayClosedGridWeeks, contentWidth), contentWidth)...)
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(theme.ColorPrimary).Italic(true).Render(dayClosedSendOff(spec)), "")
	return RenderOverlayBox(strings.Join(lines, "\n"), innerWidth, header, "", lipgloss.NewStyle().Foreground(theme.ColorSuccess))
}

// dayClosedSettled names where the pass sent the work it did not finish, each
// destination only when it received something.
func dayClosedSettled(spec DayClosedCardSpec) string {
	var parts []string
	if spec.Next > 0 {
		parts = append(parts, fmt.Sprintf("%d → Next", spec.Next))
	}
	if spec.Backlog > 0 {
		parts = append(parts, fmt.Sprintf("%d → Backlog", spec.Backlog))
	}
	if spec.StillToday > 0 {
		parts = append(parts, fmt.Sprintf("%d still in Today", spec.StillToday))
	}
	return strings.Join(parts, " · ")
}

// dayClosedSendOff is the card's last line: a clear day says so, a day with
// something finished is thanked, and a quiet one is let go without judgment.
func dayClosedSendOff(spec DayClosedCardSpec) string {
	switch {
	case len(spec.Done) > 0 && spec.StillToday == 0:
		return "Today is clear. Nice work — see you tomorrow."
	case len(spec.Done) > 0:
		return "Nice work today. See you tomorrow."
	default:
		return "Rest well. Tomorrow is a fresh start."
	}
}

// DayStartedCardSpec is the card a Start the Day pass ends on: where the pass
// sent its cards and what Today now holds.
type DayStartedCardSpec struct {
	Date         string
	Planned      int
	Rescheduled  int
	Deferred     int
	Done         int
	Today        int
	StillOverdue int
	// Doing reports work already in hand, which the card's last line names the
	// way back to.
	Doing bool
}

// RenderDayStartedCard draws the morning's closing card. Like the other cards
// it names no keys but the one way into the day: the footer under it does.
func RenderDayStartedCard(spec DayStartedCardSpec, width int) string {
	innerWidth := TriageCardInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)

	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
	label := accent.Render("Day planned")
	date := theme.MutedStyle.Render(spec.Date)
	header := label
	if gap := contentWidth - lipgloss.Width(label) - lipgloss.Width(date); spec.Date != "" && gap >= 1 {
		header += strings.Repeat(" ", gap) + date
	}

	lines := []string{triageProgressBar(1, 1, contentWidth), ""}
	switch spec.Today {
	case 0:
		lines = append(lines, theme.BoldPrimaryStyle.Render("Today holds nothing yet."))
	case 1:
		lines = append(lines, theme.BoldPrimaryStyle.Render("1 task for Today"))
	default:
		lines = append(lines, theme.BoldPrimaryStyle.Render(fmt.Sprintf("%d tasks for Today", spec.Today)))
	}
	var moves []string
	for _, move := range []struct {
		n    int
		verb string
	}{{spec.Planned, "planned"}, {spec.Rescheduled, "rescheduled"}, {spec.Deferred, "set aside"}, {spec.Done, "done"}} {
		if move.n > 0 {
			moves = append(moves, fmt.Sprintf("%d %s", move.n, move.verb))
		}
	}
	if len(moves) > 0 {
		lines = append(lines, theme.MutedStyle.Render(textfit.TruncateIntentional(strings.Join(moves, " · "), contentWidth, "…")))
	}
	if spec.StillOverdue > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(theme.ColorWarning).Render(fmt.Sprintf("%d still overdue", spec.StillOverdue)))
	}
	lines = append(lines, "")
	next := "Start one with wg; gd keeps it in front of you."
	if spec.Doing {
		next = "Work is in hand: gd brings it back in front of you."
	}
	lines = append(lines, theme.MutedStyle.Render(textfit.TruncateIntentional(next, contentWidth, "…")), "")
	return RenderOverlayBox(strings.Join(lines, "\n"), innerWidth, header, "", lipgloss.NewStyle().Foreground(theme.ColorAccent))
}

// WeekReviewedCardSpec is the card a Review the Week pass ends on: what the
// pass decided, from its trail, what the last seven days finished, and the
// half year behind as the activity grid.
type WeekReviewedCardSpec struct {
	// Range names the week read, such as "Nov 2 – Nov 8".
	Range     string
	Reviewed  int
	Planned   int
	Moved     int
	Dated     int
	Finished  int
	Canceled  int
	Kept      int
	WeekDone  int
	StillOpen int
	History   map[time.Time]int
	Now       time.Time
}

// RenderWeekReviewedCard draws the weekly pass's closing card. Like the other
// closing cards it names no keys: the footer under it does.
func RenderWeekReviewedCard(spec WeekReviewedCardSpec, width int) string {
	innerWidth := TriageCardInnerWidth(width)
	contentWidth := max(innerWidth-4, 12)

	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
	label := accent.Render("Week reviewed")
	when := theme.MutedStyle.Render(spec.Range)
	header := label
	if gap := contentWidth - lipgloss.Width(label) - lipgloss.Width(when); spec.Range != "" && gap >= 1 {
		header += strings.Repeat(" ", gap) + when
	}

	lines := []string{triageProgressBar(1, 1, contentWidth), ""}
	check := lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render("✓")
	switch spec.WeekDone {
	case 0:
		lines = append(lines, theme.MutedStyle.Render("Nothing finished in the last seven days."))
	case 1:
		lines = append(lines, check+" "+theme.BoldPrimaryStyle.Render("1 task done this week"))
	default:
		lines = append(lines, check+" "+theme.BoldPrimaryStyle.Render(fmt.Sprintf("%d tasks done this week", spec.WeekDone)))
	}
	reviewed := "Nothing needed a second look."
	if spec.Reviewed > 0 {
		reviewed = fmt.Sprintf("%d reviewed", spec.Reviewed)
		var moves []string
		for _, move := range []struct {
			n    int
			verb string
		}{
			{spec.Planned, "planned"}, {spec.Dated, "rescheduled"}, {spec.Moved, "set aside"},
			{spec.Finished, "done"}, {spec.Canceled, "canceled"}, {spec.Kept, "kept"},
		} {
			if move.n > 0 {
				moves = append(moves, fmt.Sprintf("%d %s", move.n, move.verb))
			}
		}
		if len(moves) > 0 {
			reviewed += ": " + strings.Join(moves, " · ")
		}
	}
	lines = append(lines, theme.MutedStyle.Render(textfit.TruncateIntentional(reviewed, contentWidth, "…")))
	if spec.StillOpen > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(theme.ColorWarning).Render(fmt.Sprintf("%d not yet reviewed", spec.StillOpen)))
	}
	if grid := pulse.ActivityGrid(spec.History, spec.Now, dayClosedGridWeeks, contentWidth); len(grid) > 0 {
		lines = append(lines, "")
		lines = append(lines, grid...)
		lines = append(lines, pulse.ActivityLegendLines(spec.History, spec.Now, pulse.ActivityGridWeeks(dayClosedGridWeeks, contentWidth), contentWidth)...)
	}
	sendOff := "A clean slate for the week ahead."
	if spec.StillOpen > 0 {
		sendOff = "The rest can wait for next week's look."
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(theme.ColorPrimary).Italic(true).Render(sendOff), "")
	return RenderOverlayBox(strings.Join(lines, "\n"), innerWidth, header, "", lipgloss.NewStyle().Foreground(theme.ColorAccent))
}
