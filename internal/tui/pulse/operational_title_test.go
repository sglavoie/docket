package pulse

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/theme"
)

// scopeLabel and modeBadge stand in for the shell's Active Projects label and
// its mode catalog chip. Both are chrome vocabulary that sits above this
// package, and neither spelling is what these tests assert: every expected
// width below is derived from the rendered identity, so a literal scope and a
// literal badge exercise the same fitting ladder the real ones do.
const scopeLabel = "Active Projects"

func modeBadge(label string) string {
	return OperationalTitleBadge(theme.TitleStyle, label)
}

func TestOperationalTitlePulseFittingOrder(t *testing.T) {
	pulse := WorkspacePulse{Inbox: 12, Today: 0, Overdue: 4, Done: 6}
	spec := OperationalTitleSpec{
		Surface: theme.TitleStyle.Render("Outliner"),
		Scope:   theme.TitleStyle.Render(scopeLabel),
		Mode:    modeBadge("All"),
		Pulse:   &pulse,
	}
	base := joinOperationalTitleRegions(renderOperationalIdentity(spec, 200), "")
	variants := operationalPulseVariants(pulse)
	fit := func(variant string) int {
		return lipgloss.Width(base) + operationalTitlePulseGap + lipgloss.Width(variant)
	}
	tests := []struct {
		width       int
		wantInbox   bool
		wantToday   bool
		wantOverdue bool
		wantDone    bool
	}{
		{width: lipgloss.Width(base)},
		{width: fit(variants[3]), wantToday: true},
		{width: fit(variants[2]), wantToday: true, wantOverdue: true},
		{width: fit(variants[1]), wantInbox: true, wantToday: true, wantOverdue: true},
		{width: fit(variants[0]), wantInbox: true, wantToday: true, wantOverdue: true, wantDone: true},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.width), func(t *testing.T) {
			rendered := RenderOperationalTitle(spec, tt.width)
			plain := ansi.Strip(rendered)
			if strings.Contains(rendered, "\n") {
				t.Fatalf("title wrapped structurally at width %d: %q", tt.width, plain)
			}
			if got := lipgloss.Width(rendered); got > tt.width {
				t.Fatalf("title width = %d, exceeds %d: %q", got, tt.width, plain)
			}
			for token, want := range map[string]bool{
				"inbox 12":  tt.wantInbox,
				"today 0":   tt.wantToday,
				"overdue 4": tt.wantOverdue,
				"done 6":    tt.wantDone,
			} {
				if got := strings.Contains(plain, token); got != want {
					t.Errorf("width %d token %q present = %v, want %v: %q", tt.width, token, got, want, plain)
				}
			}
		})
	}
}

func TestOperationalTitlePreservesInteractionBeforePulse(t *testing.T) {
	pulse := WorkspacePulse{Inbox: 9, Today: 8, Overdue: 7}
	rendered := RenderOperationalTitle(OperationalTitleSpec{
		Surface: "Outliner",
		Scope:   scopeLabel,
		Mode:    "All",
		Context: []string{theme.SelectionCountStyle.Render("MULTI-SELECT: 3 selected")},
		Pace:    &TodayPaceReadout{OpenCount: 8, MedianPerDay: 3},
		Pulse:   &pulse,
	}, 40)
	plain := ansi.Strip(rendered)

	if lipgloss.Width(rendered) > 40 || strings.Contains(rendered, "\n") {
		t.Fatalf("narrow interaction title did not fit one row: width=%d %q", lipgloss.Width(rendered), plain)
	}
	if !strings.Contains(plain, "MULTI-SELECT") {
		t.Fatalf("interaction context was removed before pulse: %q", plain)
	}
	if strings.Contains(plain, "workspace:") {
		t.Fatalf("passive pulse survived ahead of interaction context: %q", plain)
	}
	if strings.Contains(plain, "median") || strings.Contains(plain, "/day") {
		t.Fatalf("Today pace survived ahead of interaction context: %q", plain)
	}
}

func TestOperationalTitleTodayPaceFittingOrder(t *testing.T) {
	pulse := WorkspacePulse{Inbox: 9, Today: 8, Overdue: 7}
	pace := TodayPaceReadout{OpenCount: 8, MedianPerDay: 1.5}
	spec := OperationalTitleSpec{Scope: scopeLabel, Mode: "Today", Pace: &pace, Pulse: &pulse}
	identity := renderOperationalIdentity(spec, 200)
	paceVariants := operationalPaceVariants(pace)
	fullBase := joinOperationalTitleRegions(identity, paceVariants[0])
	mediumBase := joinOperationalTitleRegions(identity, paceVariants[1])
	shortBase := joinOperationalTitleRegions(identity, paceVariants[2])
	fullPulse := operationalPulseVariants(pulse)[0]

	tests := []struct {
		name          string
		width         int
		want          string
		forbid        []string
		wantWorkspace bool
	}{
		{
			name:          "full pace and pulse",
			width:         lipgloss.Width(fullBase) + operationalTitlePulseGap + lipgloss.Width(fullPulse),
			want:          "today 8 · median 1.5/day (30d)",
			wantWorkspace: true,
		},
		{name: "full pace after pulse", width: lipgloss.Width(fullBase), want: "today 8 · median 1.5/day (30d)", forbid: []string{"workspace:"}},
		{name: "shorten window", width: lipgloss.Width(mediumBase), want: "today 8 · median 1.5/day", forbid: []string{"(30d)", "workspace:"}},
		{name: "shorten label", width: lipgloss.Width(shortBase), want: "today 8 · 1.5/day", forbid: []string{"median", "workspace:"}},
		{name: "omit pace", width: lipgloss.Width(identity), want: scopeLabel + " · Today", forbid: []string{"/day", "workspace:"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := RenderOperationalTitle(spec, tt.width)
			plain := ansi.Strip(rendered)
			if got := lipgloss.Width(rendered); got > tt.width {
				t.Fatalf("title width = %d, exceeds %d: %q", got, tt.width, plain)
			}
			if !strings.Contains(plain, tt.want) {
				t.Fatalf("title missing %q: %q", tt.want, plain)
			}
			if got := strings.Contains(plain, "workspace:"); got != tt.wantWorkspace {
				t.Fatalf("workspace pulse present = %v, want %v: %q", got, tt.wantWorkspace, plain)
			}
			for _, forbidden := range tt.forbid {
				if strings.Contains(plain, forbidden) {
					t.Fatalf("title contains forbidden %q: %q", forbidden, plain)
				}
			}
		})
	}
}

func TestOperationalTitlePulseAndTodayPaceFitEverySupportedWidth(t *testing.T) {
	pulse := WorkspacePulse{Inbox: 9, Today: 8, Overdue: 7, Done: 6}
	pace := TodayPaceReadout{OpenCount: 8, MedianPerDay: 1.5}
	spec := OperationalTitleSpec{Scope: scopeLabel, Mode: "Today", Pace: &pace, Pulse: &pulse}
	sawPaceOnly := false
	sawPaceAndPulse := false

	for width := 40; width <= 160; width++ {
		rendered := RenderOperationalTitle(spec, width)
		plain := ansi.Strip(rendered)
		if strings.Contains(rendered, "\n") {
			t.Fatalf("width %d wrapped the title: %q", width, plain)
		}
		if got := lipgloss.Width(rendered); got > width {
			t.Fatalf("width %d rendered %d cells: %q", width, got, plain)
		}

		hasPace := strings.Contains(plain, "/day")
		hasPulse := strings.Contains(plain, "workspace:")
		if hasPulse && !strings.Contains(plain, "median 1.5/day (30d)") {
			t.Fatalf("width %d kept the pulse beside a shortened or absent pace: %q", width, plain)
		}
		if hasPace && !hasPulse {
			sawPaceOnly = true
		}
		if hasPace && hasPulse {
			sawPaceAndPulse = true
			paceEnd := strings.Index(plain, "(30d)") + len("(30d)")
			pulseStart := strings.Index(plain, "workspace:")
			if pulseStart-paceEnd < operationalTitlePulseGap {
				t.Fatalf("width %d pace and pulse collided: %q", width, plain)
			}
		}
	}

	if !sawPaceOnly || !sawPaceAndPulse {
		t.Fatalf("width sweep missed a required title state: pace-only=%v pace-and-pulse=%v", sawPaceOnly, sawPaceAndPulse)
	}
}
