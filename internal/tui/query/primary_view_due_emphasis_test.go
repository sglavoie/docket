package query

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

// FormatDueDate is the shared consumer every row-level due date comes through,
// so the emphasis a tier carries has to survive it rather than be reapplied by
// each caller. The assertion is style identity: the chip a row draws is exactly
// the chip the theme declared, which is what keeps one date from reading two
// ways on two surfaces.
func TestFormatDueDateRendersTheThemeTierUnchanged(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	today := util.StartOfDay(time.Now())
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})

			cases := []struct {
				name string
				due  time.Time
				text string
				want func() string
			}{
				{"long overdue", today.AddDate(0, 0, -9), today.AddDate(0, 0, -9).Format("Jan 2") + " overdue",
					func() string {
						return theme.OverdueStyle.Render(DueSigil + today.AddDate(0, 0, -9).Format("Jan 2") + " overdue")
					}},
				{"recently overdue", today.AddDate(0, 0, -3), "3 days ago",
					func() string { return theme.OverdueStyle.Render(DueSigil + "3 days ago") }},
				{"yesterday", today.AddDate(0, 0, -1), "Yesterday",
					func() string { return theme.OverdueStyle.Render(DueSigil + "Yesterday") }},
				{"today", today, "Today",
					func() string { return theme.DueTodayStyle.Render(DueSigil + "Today") }},
				{"tomorrow", today.AddDate(0, 0, 1), "Tomorrow",
					func() string { return theme.DueDateStyle.Render(DueSigil + "Tomorrow") }},
				{"in two days", today.AddDate(0, 0, 2), "In 2 days",
					func() string { return theme.DueDateStyle.Render(DueSigil + "In 2 days") }},
				{"later", today.AddDate(0, 0, 9), today.AddDate(0, 0, 9).Format("Jan 2"),
					func() string { return theme.DueDateStyle.Render(DueSigil + today.AddDate(0, 0, 9).Format("Jan 2")) }},
			}
			for _, tc := range cases {
				due := tc.due
				task := &models.Task{ID: tc.name, WorkflowState: models.StateBacklog, DueDate: &due}
				got := FormatDueDate(task)
				if got != tc.want() {
					t.Errorf("%s due chip = %q, want the theme tier's own render %q", tc.name, got, tc.want())
				}
				// Emphasis may not cost a column: the ladder measures the chip, and
				// --no-color leaves exactly this text behind — the value and the
				// chip's own two padding cells and the `@` that sets it, with no calendar
				// glyph.
				if plain, want := ansi.Strip(got), " "+DueSigil+tc.text+" "; plain != want {
					t.Errorf("%s due chip monochrome text = %q, want %q", tc.name, plain, want)
				}
			}

			// A timed overdue date states its clock and still reaches the overdue
			// tier, which is the one place FormatDueDate overrides the tier itself.
			// Past the relative window the text names the date, so it says overdue.
			timed := today.AddDate(0, 0, -8).Add(15*time.Hour + 30*time.Minute)
			task := &models.Task{
				ID:            "timed",
				WorkflowState: models.StateBacklog,
				DueDate:       &timed,
			}
			want := theme.OverdueStyle.Render(DueSigil + timed.Format("Jan 2") + " 15:30 overdue")
			if got := FormatDueDate(task); got != want {
				t.Errorf("timed overdue chip = %q, want %q", got, want)
			}

			// A timed deadline passed earlier today keeps the day and adds the
			// word, since "Today" alone would not say it is late.
			earlier := today.Add(time.Minute)
			if time.Now().After(earlier) {
				task := &models.Task{ID: "earlier", WorkflowState: models.StateBacklog, DueDate: &earlier}
				if got, want := ansi.Strip(FormatDueDate(task)), " @Today 00:01 overdue "; got != want {
					t.Errorf("timed deadline passed today = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestOverdueDeadlineLabelSurvivesMonochrome(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name     string
		due      time.Time
		wantLate bool
	}{
		{"passed time", now.Add(-time.Minute), true},
		{"future time", now.Add(time.Hour), false},
		{"today without time", util.StartOfDay(now), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{WorkflowState: models.StateBacklog, DueDate: &tc.due}
			plain := ansi.Strip(FormatDueDate(task))
			// A relative past day ("Yesterday", "3 days ago") already says late in
			// words; every other passed deadline spells "overdue".
			late := strings.Contains(plain, " overdue") || strings.Contains(plain, "Yesterday") || strings.Contains(plain, " ago")
			if late != tc.wantLate {
				t.Fatalf("deadline label = %q, want overdue=%v", plain, tc.wantLate)
			}
		})
	}
}
