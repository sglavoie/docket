package query

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
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
				{"overdue", today.AddDate(0, 0, -3), today.AddDate(0, 0, -3).Format("Jan 2"),
					func() string { return theme.OverdueStyle.Render(icons.Get("date") + " " + today.AddDate(0, 0, -3).Format("Jan 2")) }},
				{"today", today, "Today",
					func() string { return theme.DueTodayStyle.Render(icons.Get("date") + " Today") }},
				{"tomorrow", today.AddDate(0, 0, 1), "Tomorrow",
					func() string { return theme.DueDateStyle.Render(icons.Get("date") + " Tomorrow") }},
			}
			for _, tc := range cases {
				due := tc.due
				task := &models.Task{ID: tc.name, WorkflowState: models.StateBacklog, DueDate: &due}
				got := FormatDueDate(task)
				if got != tc.want() {
					t.Errorf("%s due chip = %q, want the theme tier's own render %q", tc.name, got, tc.want())
				}
				// Emphasis may not cost a column: the ladder measures the chip, and
				// --no-color leaves exactly this text behind — the glyph, the value,
				// and the chip's own two padding cells.
				if plain, want := ansi.Strip(got), " "+icons.Get("date")+" "+tc.text+" "; plain != want {
					t.Errorf("%s due chip monochrome text = %q, want %q", tc.name, plain, want)
				}
			}

			// A timed overdue date states its clock and still reaches the overdue
			// tier, which is the one place FormatDueDate overrides the tier itself.
			timed := today.AddDate(0, 0, -1).Add(15*time.Hour + 30*time.Minute)
			task := &models.Task{
				ID:            "timed",
				WorkflowState: models.StateBacklog,
				DueDate:       &timed,
			}
			want := theme.OverdueStyle.Render(icons.Get("date") + " " + timed.Format("Jan 2") + " 15:30")
			if got := FormatDueDate(task); got != want {
				t.Errorf("timed overdue chip = %q, want %q", got, want)
			}
		})
	}
}
