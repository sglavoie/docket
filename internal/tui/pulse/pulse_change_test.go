package pulse

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestPulseChangeBetweenNamesEachMovedCountAndItsDirection(t *testing.T) {
	before := WorkspacePulse{Inbox: 2, Today: 3, Overdue: 1, Done: 4}
	after := WorkspacePulse{Inbox: 2, Today: 2, Overdue: 1, Done: 5}
	change := PulseChangeBetween(before, after)
	if change.Raised != PulseDone || change.Lowered != PulseToday {
		t.Fatalf("change = %+v, want Done raised and Today lowered", change)
	}
	if PulseChangeBetween(before, before.Lit(change)).Any() {
		t.Fatal("an annotation alone read as a change")
	}
}

// A lit count changes paint and nothing else: the same text at the same width
// in both phases, so no ladder rung or right alignment moves.
func TestLitPulseRepaintsWithoutMovingACell(t *testing.T) {
	counts := WorkspacePulse{Inbox: 2, Today: 3, Overdue: 1, Done: 4}
	plain := formatWorkspacePulseComponents(counts, workspacePulseFull)
	change := PulseChange{Raised: PulseDone | PulseOverdue, Lowered: PulseToday}
	fresh := formatWorkspacePulseComponents(counts.Lit(change), workspacePulseFull)
	change.Settling = true
	settling := formatWorkspacePulseComponents(counts.Lit(change), workspacePulseFull)

	for name, got := range map[string]string{"fresh": fresh, "settling": settling} {
		if got == plain {
			t.Errorf("%s phase drew nothing", name)
		}
		if ansi.Strip(got) != ansi.Strip(plain) || lipgloss.Width(got) != lipgloss.Width(plain) {
			t.Errorf("%s phase moved the text: %q vs %q", name, ansi.Strip(got), ansi.Strip(plain))
		}
	}
	if fresh == settling {
		t.Error("the two phases drew alike")
	}
}
