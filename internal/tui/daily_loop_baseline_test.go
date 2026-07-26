package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestDailyLoopCurrentModeCatalogBaseline records the intentionally broad mode
// catalog and four-item Tab cycle that later sessions simplify. It is separate
// from the accepted ten-mode target fixture in internal/testutil.
func TestDailyLoopCurrentModeCatalogBaseline(t *testing.T) {
	var got []string
	for _, mode := range OutlinerModeCatalog() {
		got = append(got, mode.Label)
	}
	want := []string{
		"Next", "Backlog", "Active", "All",
		"Inbox", "Standby", "Waiting", "Someday", "Review", "Snoozed", "Completed",
		"Due Soon", "High Priority", "Blocked", "Parked", "Pinned", "Recently Visited",
		"Priority", "Due Date", "Daily Review", "Activity",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("current mode catalog = %v, want %v", got, want)
	}

	cycle := []OutlinerModeID{ModeNext, ModeBacklog, ModeActive, ModeAll}
	if !slices.Equal(workModeCycleOrder, cycle) {
		t.Errorf("current Tab cycle = %v, want %v", workModeCycleOrder, cycle)
	}
}

// TestDailyLoopCurrentScopeRestoresMode characterizes the current navigation
// stack: entering a project scope may change modes, and Esc-style scope pop
// restores the exact parent mode.
func TestDailyLoopCurrentScopeRestoresMode(t *testing.T) {
	m := newContinuityOutliner(t, 100, 30)
	_, _ = m.selectOutlinerMode(ModeBacklog)
	if len(m.loadedProjects) == 0 {
		t.Fatal("continuity fixture has no projects")
	}

	m.pushProjectScope(m.loadedProjects[0].ID)
	_, _ = m.selectOutlinerMode(ModeInbox)
	if !m.popProjectScope() {
		t.Fatal("project scope did not pop")
	}
	if m.scopedProjectID != "" {
		t.Errorf("scope after pop = %q, want all projects", m.scopedProjectID)
	}
	if got := m.activeOutlinerMode(); got != ModeBacklog {
		t.Errorf("mode after scope restore = %q, want backlog", got)
	}
}

// TestDailyLoopCurrentRepresentativeOutlinerOutput pins a compact semantic
// sample of the current TUI output before the group and mode cutover.
func TestDailyLoopCurrentRepresentativeOutlinerOutput(t *testing.T) {
	m, _ := crossProjectRenderFixture(t, 112, 18)
	plain := ansi.Strip(m.renderOutlinerTreeContent(112))
	for _, want := range []string{"Outliner (2 projects)", "@ All", "@ Inbox", "@ Next", "@ Active"} {
		if !strings.Contains(plain, want) {
			t.Errorf("representative Outliner output is missing %q:\n%s", want, plain)
		}
	}
}
