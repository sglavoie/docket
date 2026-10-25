package chrome

import (
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

// ClassifyDueDate is the one place a calendar day becomes a tier, so every
// surface that shows a due date inherits its answer. What has to hold is that a
// deadline which has arrived or passed reaches an emphasized tier and one the
// user still has time for does not, under both themes: the chips are repainted
// per theme, and a tier resolved before the repaint would strand a row on the
// outgoing look.
func TestClassifyDueDateTiersCarryTheirEmphasis(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	today := util.StartOfDay(time.Now())
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})

			cases := []struct {
				name       string
				due        time.Time
				want       lipgloss.Style
				emphasized bool
			}{
				{"overdue", today.AddDate(0, 0, -3), theme.OverdueStyle, true},
				{"today", today.Add(9 * time.Hour), theme.DueTodayStyle, true},
				{"tomorrow", today.AddDate(0, 0, 1), theme.DueDateStyle, false},
				{"later", today.AddDate(0, 0, 9), theme.DueDateStyle, false},
			}
			for _, tc := range cases {
				due := tc.due
				text, style := ClassifyDueDate(&due)
				if text == "" {
					t.Fatalf("%s due date rendered no text", tc.name)
				}
				if got, want := style.GetBackground(), tc.want.GetBackground(); got != want {
					t.Errorf("%s due date fill = %v, want %v", tc.name, got, want)
				}
				if got, want := style.GetForeground(), tc.want.GetForeground(); got != want {
					t.Errorf("%s due date ink = %v, want %v", tc.name, got, want)
				}
				if got := style.GetBold(); got != tc.emphasized {
					t.Errorf("%s due date emphasized = %v, want %v", tc.name, got, tc.emphasized)
				}
			}

			if text, style := ClassifyDueDate(nil); text != "" || style.GetBold() {
				t.Errorf("absent due date classified as %q with bold = %v", text, style.GetBold())
			}
		})
	}
}

func TestClassifyDueDateKeepsYearOnOldOverdueDates(t *testing.T) {
	now := time.Now()
	old := time.Date(now.Year()-1, time.January, 2, 0, 0, 0, 0, time.Local)
	text, style := ClassifyDueDate(&old)
	if text != old.Format("Jan 2 '06") || style.GetForeground() != theme.OverdueStyle.GetForeground() {
		t.Fatalf("old deadline lost year or overdue styling: %q", text)
	}
}
