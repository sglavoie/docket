package tui

import (
	"strings"
	"testing"
)

// How the outstanding-conflict count renders. Counting the bundles it reports
// is the shell's job; see conflict_indicator_shell_test.go.

func TestConflictIndicatorTextRendersNothingBelowOne(t *testing.T) {
	for _, count := range []int{-1, 0} {
		if got := conflictIndicatorText(count); got != "" {
			t.Fatalf("indicator text for %d = %q, want empty", count, got)
		}
	}
	if got := conflictIndicatorText(1); got != "1 unresolved merge conflict — review with tt conflict list" {
		t.Fatalf("singular indicator text = %q", got)
	}
	if got := conflictIndicatorText(4); got != "4 unresolved merge conflicts — review with tt conflict list" {
		t.Fatalf("plural indicator text = %q", got)
	}
}

// TestConflictIndicatorRendersInTheStatusStrip pins the placement decision: the
// indicator is a status-strip line, so it appears in the strip body, reserves
// height through statusStripHeight, and leads the transient lines below it.
func TestConflictIndicatorRendersInTheStatusStrip(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	const width = 80
	m := &OutlinerModel{}

	if body := m.statusStripBody(width); body != "" {
		t.Fatalf("strip body with no injected accessor = %q, want empty", body)
	}
	if height := m.statusStripHeight(width); height != 0 {
		t.Fatalf("strip height with no injected accessor = %d, want 0", height)
	}

	count := 0
	m.setOutstandingConflicts(func() int { return count })
	if body := m.statusStripBody(width); body != "" {
		t.Fatalf("strip body with zero conflicts = %q, want empty", body)
	}

	count = 2
	body := m.statusStripBody(width)
	if !strings.Contains(body, "2 unresolved merge conflicts") {
		t.Fatalf("strip body with two conflicts = %q", body)
	}
	if height := m.statusStripHeight(width); height < 2 {
		t.Fatalf("strip height with an indicator = %d, want the divider plus its line", height)
	}

	// The indicator is durable state and the status message is transient, so the
	// indicator must stay on the first row rather than being pushed down by a
	// message that will clear on its own.
	m.setStatus("Moved 1 task")
	lines := strings.Split(m.statusStripBody(width), "\n")
	if len(lines) != 2 {
		t.Fatalf("strip lines = %d, want the indicator plus the status message", len(lines))
	}
	if !strings.Contains(lines[0], "2 unresolved merge conflicts") {
		t.Fatalf("first strip line = %q, want the indicator", lines[0])
	}
	if !strings.Contains(lines[1], "Moved 1 task") {
		t.Fatalf("second strip line = %q, want the status message", lines[1])
	}

	// Resolving the last bundle clears the line and gives the rows back.
	count = 0
	m.clearStatus()
	if body := m.statusStripBody(width); body != "" {
		t.Fatalf("strip body after the conflicts were resolved = %q, want empty", body)
	}
	if height := m.statusStripHeight(width); height != 0 {
		t.Fatalf("strip height after the conflicts were resolved = %d, want 0", height)
	}
}
