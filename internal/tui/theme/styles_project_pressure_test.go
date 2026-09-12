package theme

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// The pressure counts are the header's half of the deadline emphasis, so what
// has to hold of them is a pair property: same weight, different color, and
// neither color invented here.
func TestProjectPressureCountsShareWeightAndKeepTheirHues(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: themeName})

			counts := map[string]lipgloss.Style{
				"overdue": ProjectPressureOverdueStyle,
				"today":   ProjectPressureTodayStyle,
			}
			for name, style := range counts {
				if !style.GetBold() {
					t.Errorf("%s pressure count is not emphasized, so a folded group understates it", name)
				}
				if style.GetForeground() == (lipgloss.NoColor{}) {
					t.Errorf("%s pressure count carries no foreground", name)
				}
				if style.GetBackground() != (lipgloss.NoColor{}) {
					t.Errorf("%s pressure count carries fill %v, so it reads as a second badge",
						name, style.GetBackground())
				}
				if left, right := style.GetPaddingLeft(), style.GetPaddingRight(); left != 0 || right != 0 {
					t.Errorf("%s pressure count pads %d/%d, which the header's budget never measured",
						name, left, right)
				}
			}

			if counts["overdue"].GetForeground() == counts["today"].GetForeground() {
				t.Fatalf("both pressure counts wear %v, so a plan reads as a deadline",
					counts["overdue"].GetForeground())
			}
			if got, want := counts["overdue"].GetForeground(), LensAccentOverdueStyle.GetForeground(); got != want {
				t.Errorf("overdue count color = %v, want the due family's own accent %v", got, want)
			}
			todayAccent, ok := WorkflowStateGroupAccentStyle(models.StateGroupToday)
			if !ok {
				t.Fatal("the Today group has no accent, so the today count has nothing to borrow")
			}
			if got, want := counts["today"].GetForeground(), todayAccent.GetForeground(); got != want {
				t.Errorf("today count color = %v, want the Today group's accent %v", got, want)
			}
		})
	}
}

// The accents the pair borrows are themselves rebuilt on every ApplyTheme, so
// the pair has to be rebuilt after them; a stale copy would leave a light-theme
// header wearing the dark theme's counts.
func TestProjectPressureCountsTrackTheActiveTheme(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	ApplyTheme(ThemeConfig{Name: "dark"})
	darkOverdue := ProjectPressureOverdueStyle.GetForeground()
	darkToday := ProjectPressureTodayStyle.GetForeground()

	ApplyTheme(ThemeConfig{Name: "light"})
	if got := ProjectPressureOverdueStyle.GetForeground(); got == darkOverdue {
		t.Errorf("overdue count kept its dark color %v under the light theme", got)
	}
	if got := ProjectPressureTodayStyle.GetForeground(); got == darkToday {
		t.Errorf("today count kept its dark color %v under the light theme", got)
	}
}
