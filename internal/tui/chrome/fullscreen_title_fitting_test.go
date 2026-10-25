package chrome

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/pulse"
)

// TestFullscreenTitleChromeFittingStaysWithinWidth verifies that the semantic
// title renderer keeps full-screen chrome on one bounded row at both narrow and
// wide terminal sizes.
func TestFullscreenTitleChromeFittingStaysWithinWidth(t *testing.T) {
	longPrefix := strings.Repeat("Fullscreen Title Prefix ", 6)
	for _, tc := range []struct {
		name  string
		title string
		width int
		pulse pulse.WorkspacePulse
	}{
		{name: "narrow", title: longPrefix, width: 20, pulse: pulse.WorkspacePulse{}},
		{name: "wide", title: "Tasks", width: 120, pulse: pulse.WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rendered := RenderTaskScopedFullscreenTitleWithPulse(tc.title, nil, "", tc.pulse, tc.width)
			if strings.Contains(rendered, "\n") {
				t.Fatalf("fullscreen title wrapped onto multiple rows: %q", rendered)
			}
			if got := lipgloss.Width(rendered); got > tc.width {
				t.Fatalf("fullscreen title width = %d, want <= %d: %q", got, tc.width, rendered)
			}
			if tc.name == "wide" && !strings.Contains(rendered, "done 4") {
				t.Fatalf("wide fullscreen title omitted Done progress: %q", rendered)
			}
		})
	}
}
