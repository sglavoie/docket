package pulse

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestActivityGridDrawsWeeksAndMarksToday(t *testing.T) {
	now := time.Date(2026, 11, 8, 15, 0, 0, 0, time.Local) // a Sunday
	history := map[time.Time]int{
		time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local): 3,
		time.Date(2026, 11, 3, 0, 0, 0, 0, time.Local): 1,
		time.Date(2026, 10, 5, 0, 0, 0, 0, time.Local): 6,
	}
	lines := ActivityGrid(history, now, 26, 80)
	if len(lines) != 8 {
		t.Fatalf("got %d lines, want 7 weekday rows and a month row", len(lines))
	}
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = ansi.Strip(line)
		if w := ansi.StringWidth(line); w > 80 {
			t.Fatalf("line %d is %d wide", i, w)
		}
	}
	if !strings.HasPrefix(plain[0], "Mon ") || !strings.HasPrefix(plain[2], "Wed ") {
		t.Fatalf("weekday labels missing:\n%s", strings.Join(plain, "\n"))
	}
	if got := strings.Count(strings.Join(plain[:7], ""), "■"); got != 3 {
		t.Fatalf("got %d active cells, want 3:\n%s", got, strings.Join(plain, "\n"))
	}
	// Sunday the 8th is today and finished nothing: it is the last cell of the
	// last row, marked rather than dotted.
	if !strings.HasSuffix(plain[6], "□") {
		t.Fatalf("today is not marked:\n%s", strings.Join(plain, "\n"))
	}
	if !strings.Contains(plain[7], "Oct") || !strings.Contains(plain[7], "Nov") {
		t.Fatalf("month row lacks Oct and Nov: %q", plain[7])
	}
}

func TestActivityGridDrawsNothingWithoutHistoryOrRoom(t *testing.T) {
	now := time.Date(2026, 11, 8, 15, 0, 0, 0, time.Local)
	if lines := ActivityGrid(nil, now, 26, 80); lines != nil {
		t.Fatal("an empty history drew a grid")
	}
	history := map[time.Time]int{now: 1}
	if lines := ActivityGrid(history, now, 26, ActivityGridMinWidth-1); lines != nil {
		t.Fatal("a grid drew below its minimum width")
	}
}

func TestCompletionStreakToleratesAnUnfinishedToday(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.Local)
	day := func(offset int) time.Time { return time.Date(2026, 11, 8+offset, 0, 0, 0, 0, time.Local) }
	history := map[time.Time]int{day(-1): 2, day(-2): 1, day(-3): 4, day(-5): 1}
	if got := CompletionStreak(history, now); got != 3 {
		t.Fatalf("streak = %d, want 3 counted back from yesterday", got)
	}
	history[day(0)] = 1
	if got := CompletionStreak(history, now); got != 4 {
		t.Fatalf("streak = %d, want 4 including today", got)
	}
	legend := ansi.Strip(strings.Join(ActivityLegendLines(history, now, 26, 200), "\n"))
	if !strings.Contains(legend, "9 done in 26 weeks") || !strings.Contains(legend, "4-day streak") || !strings.Contains(legend, "best day 4") {
		t.Fatalf("legend = %q", legend)
	}
}

// The week strip's load row stands a bar under every dated day, tallest on the
// busiest, and nothing under an empty one.
func TestAheadLoadRowScalesToTheBusiestDay(t *testing.T) {
	canvas := DayCanvas{Ahead: []HorizonDay{{Due: 4}, {}, {Due: 1, CheckIns: 1}}}
	row := ansi.Strip(canvas.aheadLoadRow(10))
	if !strings.Contains(row, "███") || !strings.Contains(row, "▄▄▄") {
		t.Fatalf("load row = %q, want a full bar for 4 and a half bar for 2", row)
	}
	if strings.Index(row, "▄") < strings.Index(row, "█")+10 {
		t.Fatalf("the empty day drew a bar: %q", row)
	}
}
