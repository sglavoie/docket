package chrome

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// RenderBlockClock draws five equal-width rows. Elapsed time is H:MM;
// remaining time is minutes rounded up, with at least two digits.
func RenderBlockClock(duration time.Duration, remaining bool) string {
	minutes := clockMinutes(duration, remaining)
	reading := fmt.Sprintf("%d:%02d", minutes/60, minutes%60)
	if remaining {
		reading = fmt.Sprintf("%02d", minutes)
	}
	var rows [5]string
	for _, digit := range reading {
		glyph := blockClockGlyphs[digit]
		for row := range rows {
			if rows[row] != "" {
				rows[row] += " "
			}
			rows[row] += glyph[row]
		}
	}
	return lipgloss.NewStyle().Foreground(theme.ColorAccent).Render(strings.Join(rows[:], "\n"))
}

func clockMinutes(duration time.Duration, remaining bool) int64 {
	duration = max(duration, 0)
	minutes := int64(duration / time.Minute)
	if remaining && duration%time.Minute != 0 {
		minutes++
	}
	return minutes
}

var blockClockGlyphs = map[rune][5]string{
	'0': {"███", "█ █", "█ █", "█ █", "███"},
	'1': {" █ ", "██ ", " █ ", " █ ", "███"},
	'2': {"███", "  █", "███", "█  ", "███"},
	'3': {"███", "  █", "███", "  █", "███"},
	'4': {"█ █", "█ █", "███", "  █", "  █"},
	'5': {"███", "█  ", "███", "  █", "███"},
	'6': {"███", "█  ", "███", "█ █", "███"},
	'7': {"███", "  █", "  █", "  █", "  █"},
	'8': {"███", "█ █", "███", "█ █", "███"},
	'9': {"███", "█ █", "███", "  █", "███"},
	':': {" ", "█", " ", "█", " "},
}

func nowFocusBar(focus NowScreenFocus, width int) string {
	spent := 0
	if focus.Length > 0 {
		fraction := 1 - float64(max(focus.Remaining, 0))/float64(focus.Length)
		spent = min(max(int(fraction*float64(width)), 0), width)
	}
	return lipgloss.NewStyle().Foreground(theme.ColorAccent).Render(strings.Repeat("━", spent)) +
		theme.MutedStyle.Render(strings.Repeat("─", width-spent))
}
