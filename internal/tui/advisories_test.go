package tui

import (
	"strings"
	"testing"
)

// TestAdvisoryLineDropsTheStripRowAndItsHeight pins the shape of the opt-out: a
// suppressed advisory is not dimmed or shortened, it is absent, and the row it
// would have taken goes back to the list. Height and body read the same helper,
// so the reservation follows without a second measurement.
func TestAdvisoryLineDropsTheStripRowAndItsHeight(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	const width = 80
	m := &OutlinerModel{}
	m.setOutstandingConflicts(func() int { return 2 })

	if body := m.statusStripBody(width); !strings.Contains(body, "2 unresolved merge conflicts") {
		t.Fatalf("baseline strip body = %q, want the indicator", body)
	}
	shown := m.statusStripHeight(width)
	if shown < 2 {
		t.Fatalf("baseline strip height = %d, want the divider plus its line", shown)
	}

	hidden := true
	m.setAdvisoriesHidden(func() bool { return hidden })
	if body := m.statusStripBody(width); body != "" {
		t.Fatalf("suppressed strip body = %q, want empty", body)
	}
	if height := m.statusStripHeight(width); height != 0 {
		t.Fatalf("suppressed strip height = %d, want 0", height)
	}

	// Feedback from the user's own operations is not an advisory, so hiding
	// advisories must never silence it — and the indicator must stay gone above it.
	m.setStatus("Moved 1 task")
	body := m.statusStripBody(width)
	if !strings.Contains(body, "Moved 1 task") {
		t.Fatalf("suppressed strip dropped the user's own status: %q", body)
	}
	if strings.Contains(body, "unresolved merge conflict") {
		t.Fatalf("the indicator came back beside a status message: %q", body)
	}

	hidden = false
	m.clearStatus()
	if got := m.statusStripHeight(width); got != shown {
		t.Fatalf("strip height after showing again = %d, want the baseline %d", got, shown)
	}
}

// TestAdvisoryHelpersAreNilSafeAndDefaultToShowing keeps the injected accessor
// safe for the construction sites that own no shell — a missing accessor must
// read as showing, so nothing here can turn a struct-literal model silent.
func TestAdvisoryHelpersAreNilSafeAndDefaultToShowing(t *testing.T) {
	var shell *AppModel
	if shell.advisoriesHidden() {
		t.Fatal("a nil shell reports advisories hidden")
	}

	m := &OutlinerModel{}
	if m.advisoriesSuppressed() {
		t.Fatal("a model with no injected accessor suppresses advisories")
	}
	if got := m.advisoryLine("text"); got != "text" {
		t.Fatalf("advisoryLine with no accessor = %q, want the text through", got)
	}
	if m.advisoryStatus("text") == nil {
		t.Fatal("advisoryStatus with no accessor returned nil")
	}
	if m.advisoryStatus("") != nil {
		t.Fatal("advisoryStatus with empty text must send nothing")
	}
}

// TestLoadFailureAdvisoryObeysTheGate covers the second ambient producer: the
// partly-loaded-population notice. It routes through the same helper as the strip
// row, which is what makes one flag cover both.
func TestLoadFailureAdvisoryObeysTheGate(t *testing.T) {
	failures := []projectLoadFailure{{Detail: "alpha: unreadable"}}

	m := &OutlinerModel{}
	if m.loadFailureAdvisory(nil) != nil {
		t.Fatal("no failures must produce no notice")
	}
	cmd := m.loadFailureAdvisory(failures)
	if cmd == nil {
		t.Fatal("a failed load produced no notice while advisories were shown")
	}
	msg, ok := cmd().(InfoStatusMsg)
	if !ok {
		t.Fatalf("notice message = %T, want InfoStatusMsg", cmd())
	}
	if !strings.Contains(string(msg), "1 project failed to load") {
		t.Fatalf("notice text = %q", string(msg))
	}

	m.setAdvisoriesHidden(func() bool { return true })
	if m.loadFailureAdvisory(failures) != nil {
		t.Fatal("a suppressed load-failure notice was still sent")
	}
}
