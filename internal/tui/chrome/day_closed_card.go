package chrome

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

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
}

// dayClosedMaxDone bounds the finished titles listed before counting the rest,
// the same five the day canvas lists.
const dayClosedMaxDone = 5

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
				slug = theme.MutedStyle.Render(" [" + task.ProjectSlug + "]")
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
