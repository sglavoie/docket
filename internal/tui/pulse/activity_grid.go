package pulse

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// The activity grid is the completion history drawn the way a contribution
// graph is: one column per week, Monday at the top, each day's Done count as
// a cell whose ink deepens with the count. It is a reading of stored
// CompletedAt instants and writes nothing; a workspace with no history draws
// nothing, since an empty grid would only say so at length.

// activityGridLevels is how many inks the ramp has above an empty day.
const activityGridLevels = 4

// activityGridLabelWidth is the weekday label column, "Mon " included.
const activityGridLabelWidth = 4

// activityGridCell is one day's width: the mark and the gap after it.
const activityGridCell = 2

// ActivityGridMinWidth is the narrowest width the grid draws at, enough for
// eight weeks; below it ActivityGrid returns nothing.
const ActivityGridMinWidth = activityGridLabelWidth + 8*activityGridCell

// ActivityGrid renders up to weeks columns of history ending in now's week,
// within width, as eight lines: seven weekday rows and the month row beneath.
// It returns nil when history holds nothing or width cannot hold eight weeks.
func ActivityGrid(history map[time.Time]int, now time.Time, weeks, width int) []string {
	if len(history) == 0 || width < ActivityGridMinWidth || weeks <= 0 {
		return nil
	}
	weeks = ActivityGridWeeks(weeks, width)
	byDay := activityByDay(history, now.Location())
	today := activityDay(now, now.Location())
	firstMonday := WeekStart(now).AddDate(0, 0, -7*(weeks-1))

	peak := 0
	for day, count := range byDay {
		if !day.Before(firstMonday) && !day.After(today) {
			peak = max(peak, count)
		}
	}
	if peak == 0 {
		return nil
	}

	empty := lipgloss.NewStyle().Foreground(theme.ColorDimmed)
	label := theme.MutedStyle
	todayMark := lipgloss.NewStyle().Foreground(theme.ColorPrimary).Bold(true)
	lines := make([]string, 0, 8)
	for weekday := 0; weekday < 7; weekday++ {
		var row strings.Builder
		name := ""
		if weekday%2 == 0 && weekday < 6 {
			name = firstMonday.AddDate(0, 0, weekday).Format("Mon")
		}
		row.WriteString(label.Render(fmt.Sprintf("%-*s", activityGridLabelWidth, name)))
		for week := 0; week < weeks; week++ {
			day := firstMonday.AddDate(0, 0, 7*week+weekday)
			switch {
			case day.After(today):
				row.WriteString(strings.Repeat(" ", activityGridCell))
			case byDay[day] == 0 && day.Equal(today):
				row.WriteString(todayMark.Render("□") + " ")
			case byDay[day] == 0:
				row.WriteString(empty.Render("·") + " ")
			default:
				level := (byDay[day]*activityGridLevels + peak - 1) / peak
				ink := theme.ActivityLevelColor(theme.ColorSuccess, level, activityGridLevels)
				row.WriteString(lipgloss.NewStyle().Foreground(ink).Render("■") + " ")
			}
		}
		lines = append(lines, strings.TrimRight(row.String(), " "))
	}
	lines = append(lines, activityMonthRow(firstMonday, weeks, width))
	return lines
}

// activityMonthRow names each month over the first week column it opens,
// skipping a name that would collide with the one before it. A name may run
// past the last column into the width beyond it, so the newest month is named.
func activityMonthRow(firstMonday time.Time, weeks, width int) string {
	row := []rune(strings.Repeat(" ", max(width, activityGridLabelWidth+weeks*activityGridCell)))
	free := 0
	for week := 0; week < weeks; week++ {
		monday := firstMonday.AddDate(0, 0, 7*week)
		if week > 0 && monday.AddDate(0, 0, -7).Month() == monday.Month() {
			continue
		}
		at := activityGridLabelWidth + week*activityGridCell
		name := []rune(monday.Format("Jan"))
		if at < free || at+len(name) > len(row) {
			continue
		}
		copy(row[at:], name)
		free = at + len(name) + 1
	}
	return theme.MutedStyle.Render(strings.TrimRight(string(row), " "))
}

// ActivityGridWeeks is how many of weeks the grid draws within width. One
// column beyond the last cell stays free, since the newest month's name is a
// rune wider than its cell.
func ActivityGridWeeks(weeks, width int) int {
	return max(min(weeks, (width-activityGridLabelWidth-1)/activityGridCell), 0)
}

// ActivityLegendLines is ActivityLegend within width: one line when the ramp
// and the reading fit side by side, else the reading under the ramp.
func ActivityLegendLines(history map[time.Time]int, now time.Time, weeks, width int) []string {
	ramp, reading := activityLegendParts(history, now, weeks)
	if lipgloss.Width(ramp)+3+lipgloss.Width(reading) <= width {
		return []string{ramp + theme.MutedStyle.Render("   "+reading)}
	}
	return []string{ramp, theme.MutedStyle.Render(reading)}
}

// ActivityLegend is the grid's key and its one-line reading: the ramp from
// less to more, then the total over the grid's weeks, the best day, and the
// current streak when it is more than a day.
func ActivityLegend(history map[time.Time]int, now time.Time, weeks int) string {
	ramp, reading := activityLegendParts(history, now, weeks)
	return ramp + theme.MutedStyle.Render("   "+reading)
}

func activityLegendParts(history map[time.Time]int, now time.Time, weeks int) (string, string) {
	byDay := activityByDay(history, now.Location())
	firstMonday := WeekStart(now).AddDate(0, 0, -7*(weeks-1))
	total, best := 0, 0
	for day, count := range byDay {
		if day.Before(firstMonday) {
			continue
		}
		total += count
		best = max(best, count)
	}
	var ramp strings.Builder
	ramp.WriteString(theme.MutedStyle.Render("less "))
	ramp.WriteString(lipgloss.NewStyle().Foreground(theme.ColorDimmed).Render("·"))
	for level := 1; level <= activityGridLevels; level++ {
		ramp.WriteString(" " + lipgloss.NewStyle().Foreground(theme.ActivityLevelColor(theme.ColorSuccess, level, activityGridLevels)).Render("■"))
	}
	ramp.WriteString(theme.MutedStyle.Render(" more"))
	parts := []string{fmt.Sprintf("%d done in %d weeks", total, weeks)}
	if best > 1 {
		parts = append(parts, fmt.Sprintf("best day %d", best))
	}
	if streak := CompletionStreak(history, now); streak > 1 {
		parts = append(parts, fmt.Sprintf("%d-day streak", streak))
	}
	return ramp.String(), strings.Join(parts, " · ")
}

// CompletionStreak is how many consecutive days, ending today, finished
// something. A today with nothing done yet does not break it: the run is then
// counted back from yesterday, since the day is not over.
func CompletionStreak(history map[time.Time]int, now time.Time) int {
	byDay := activityByDay(history, now.Location())
	day := activityDay(now, now.Location())
	if byDay[day] == 0 {
		day = day.AddDate(0, 0, -1)
	}
	streak := 0
	for byDay[day] > 0 {
		streak++
		day = day.AddDate(0, 0, -1)
	}
	return streak
}

// activityByDay keys history by local calendar midnight in loc, so a key's
// own clock and zone never decide which day it is.
func activityByDay(history map[time.Time]int, loc *time.Location) map[time.Time]int {
	byDay := make(map[time.Time]int, len(history))
	for day, count := range history {
		if count > 0 {
			byDay[activityDay(day, loc)] += count
		}
	}
	return byDay
}

func activityDay(t time.Time, loc *time.Location) time.Time {
	local := t.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}
