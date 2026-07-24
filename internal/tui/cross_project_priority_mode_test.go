package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// TestEnterCrossPriorityModeSeedsBlockedCollapsed verifies that entering the
// cross-project Priority mode seeds the collapsed-groups map with
// PriorityGroupWaitingOn = true so Blocked/Waiting starts collapsed.
func TestEnterCrossPriorityModeSeedsBlockedCollapsed(t *testing.T) {
	m := minimalOutliner()

	if m.crossPriorityMode {
		t.Fatal("crossPriorityMode should default to false")
	}
	if m.crossPriorityCollapsedGroups != nil {
		t.Fatal("crossPriorityCollapsedGroups should be nil before entering mode")
	}

	m.enterCrossPriorityMode()

	if !m.crossPriorityMode {
		t.Fatal("enterCrossPriorityMode should set crossPriorityMode to true")
	}
	if !m.crossPriorityCollapsedGroups[PriorityGroupWaitingOn] {
		t.Errorf("Blocked/Waiting group should be collapsed by default, got map=%v", m.crossPriorityCollapsedGroups)
	}
	// Only Blocked/Waiting should be seeded; other groups must remain false.
	for _, g := range CrossPriorityGroupOrder() {
		if g == PriorityGroupWaitingOn {
			continue
		}
		if m.crossPriorityCollapsedGroups[g] {
			t.Errorf("group %v should not be collapsed on entry, got true", g)
		}
	}
	if !m.crossForceRebuild {
		t.Error("enterCrossPriorityMode should set crossForceRebuild=true to invalidate memoization")
	}
}

// TestExitCrossPriorityModeClearsCollapsedGroups verifies that exiting the
// mode resets the session-only collapsed map so a re-entry can re-seed
// Blocked/Waiting fresh.
func TestExitCrossPriorityModeClearsCollapsedGroups(t *testing.T) {
	m := minimalOutliner()
	m.enterCrossPriorityMode()

	// Simulate user manually collapsing another group during the session.
	m.crossPriorityCollapsedGroups[PriorityGroupLater] = true

	m.exitCrossPriorityMode()

	if m.crossPriorityMode {
		t.Fatal("exitCrossPriorityMode should set crossPriorityMode to false")
	}
	if m.crossPriorityCollapsedGroups != nil {
		t.Errorf("crossPriorityCollapsedGroups should be cleared on exit, got %v", m.crossPriorityCollapsedGroups)
	}
	if !m.crossForceRebuild {
		t.Error("exitCrossPriorityMode should set crossForceRebuild=true to invalidate memoization")
	}
}

// TestToggleCrossPriorityModeRoundTripsAndReseedsBlocked verifies that the
// public toggle alternates the mode and, on re-entry after a manual expand,
// re-seeds Blocked/Waiting collapsed.
func TestToggleCrossPriorityModeRoundTripsAndReseedsBlocked(t *testing.T) {
	m := minimalOutliner()

	m.toggleCrossPriorityMode()
	if !m.crossPriorityMode {
		t.Fatal("toggle from off should turn priority mode on")
	}
	if !m.crossPriorityCollapsedGroups[PriorityGroupWaitingOn] {
		t.Fatal("first entry should seed Blocked/Waiting collapsed")
	}

	// User expands Blocked/Waiting during the session, then exits.
	m.crossPriorityCollapsedGroups[PriorityGroupWaitingOn] = false
	m.toggleCrossPriorityMode()
	if m.crossPriorityMode {
		t.Fatal("toggle from on should turn priority mode off")
	}

	// Re-enter: Blocked/Waiting must be collapsed again because the map is
	// session-only and re-seeded on every entry.
	m.toggleCrossPriorityMode()
	if !m.crossPriorityMode {
		t.Fatal("second toggle should re-enter priority mode")
	}
	if !m.crossPriorityCollapsedGroups[PriorityGroupWaitingOn] {
		t.Error("re-entry should re-seed Blocked/Waiting collapsed, got expanded")
	}
}

// TestEscCascadePrioritizesLocalDismissalsBeforePriorityExit verifies the
// EscStep ordering: multi-select, search, zoom, and finally crossPriorityMode.
func TestEscCascadePrioritizesLocalDismissalsBeforePriorityExit(t *testing.T) {
	m := minimalOutliner()
	m.MultiSelectState = MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)}
	m.enterCrossPriorityMode()
	m.toggleMultiSelectMode()
	if !m.multiSelectMode {
		t.Fatal("expected multi-select to be active before Esc")
	}

	// First Esc: multi-select wins, priority mode stays on.
	RunEscCascade(m.escSteps())
	if m.multiSelectMode {
		t.Error("multi-select should clear before priority mode exits")
	}
	if !m.crossPriorityMode {
		t.Error("priority mode should still be on after first Esc clears multi-select")
	}

	// Add a search query and a zoom-stack entry; cascade must drain those
	// before falling through to priority mode exit.
	m.SetSearchQuery("alpha")
	m.crossZoomStack = append(m.crossZoomStack, CrossProjectZoomEntry{TaskID: "t", ProjectID: "p", CursorIndex: 0})

	RunEscCascade(m.escSteps())
	if m.SearchQuery() != "" {
		t.Error("search query should clear before priority mode exits")
	}
	if !m.crossPriorityMode {
		t.Error("priority mode should still be on after second Esc clears search")
	}

	RunEscCascade(m.escSteps())
	if len(m.crossZoomStack) != 0 {
		t.Error("cross zoom stack should drain before priority mode exits")
	}
	if !m.crossPriorityMode {
		t.Error("priority mode should still be on after third Esc clears zoom stack")
	}

	// Final Esc: nothing local is left to dismiss, so priority mode exits.
	RunEscCascade(m.escSteps())
	if m.crossPriorityMode {
		t.Error("priority mode should exit on the final Esc when no local dismissals remain")
	}
}

// TestEscCascadeOnlyAffectsCrossProjectMode verifies that the priority-exit
// EscStep is gated on m.project == nil. In single-project mode the cascade
// must not touch crossPriorityMode even if it is somehow set.
func TestEscCascadeOnlyAffectsCrossProjectMode(t *testing.T) {
	m := minimalOutliner()
	// Pretend a single-project model also has the cross priority flag set
	// (a contrived state). The cascade must ignore it.
	m.project = &models.Project{ID: "p1", Name: "P1"}
	m.crossPriorityMode = true

	RunEscCascade(m.escSteps())

	if !m.crossPriorityMode {
		t.Error("single-project Esc cascade must not clear cross-project priority mode")
	}
}

// TestCrossPriorityModeBypassesSortField verifies that the urgency-based row
// ordering ignores m.sort.Field while in priority mode, so a non-default sort
// configured before mode entry does not perturb the priority grouping.
func TestCrossPriorityModeBypassesSortField(t *testing.T) {
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))

	high := makePriorityTask("high", "High prio", intPtr(0))
	high.Priority = 1 // highest urgency
	low := makePriorityTask("low", "Low prio", intPtr(0))
	low.Priority = 5 // lowest urgency

	proj := &models.Project{ID: "proj-sort", Name: "Sort", Tasks: []*models.Task{low, high}}
	proj.BuildTaskTree()
	m.loadedProjects = []*models.Project{proj}

	// Apply a non-default sort that would otherwise reorder the rows.
	m.sort = SortState{Field: SortByPriority, Ascending: false}

	m.enterCrossPriorityMode()
	m.crossProjectRebuildRows()

	var ids []string
	for _, row := range m.rows {
		if row.task != nil {
			ids = append(ids, row.task.ID)
		}
	}
	// Urgency sort places the higher-priority task first regardless of
	// m.sort. Both tasks share the Today bucket so order is purely urgency.
	if len(ids) != 2 {
		t.Fatalf("expected 2 task rows, got %d (%v)", len(ids), ids)
	}
	if ids[0] != "high" || ids[1] != "low" {
		t.Errorf("priority mode row order should follow urgency, got %v", ids)
	}
}

// TestCrossPriorityModeTitleRendering verifies the title prefix swap from
// "Outliner" to "Outliner: Priority" while in priority mode.
func TestCrossPriorityModeTitleRendering(t *testing.T) {
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(120, 24)

	proj := &models.Project{ID: "proj-title", Name: "Title"}
	proj.Tasks = []*models.Task{makePriorityTask("t1", "Sample", intPtr(0))}
	proj.BuildTaskTree()
	m.loadedProjects = []*models.Project{proj}

	// Off: title uses the default outliner prefix.
	off := ansi.Strip(m.renderOutlinerTitle(120))
	if !strings.Contains(off, "Outliner (1 project)") {
		t.Errorf("expected default outliner title, got %q", off)
	}
	if strings.Contains(off, "Outliner: Priority") {
		t.Errorf("default title should not advertise Priority mode, got %q", off)
	}

	// On: title swaps to the Priority prefix while preserving the project count.
	m.enterCrossPriorityMode()
	on := ansi.Strip(m.renderOutlinerTitle(120))
	if !strings.Contains(on, "Outliner: Priority (1 project)") {
		t.Errorf("expected Priority-mode title with project count, got %q", on)
	}
}

// TestCrossPriorityModeSuppressesPerspectiveAndSort verifies that, when active
// cross-project Priority mode is entered from a non-All perspective with a
// non-default sort and an active filter+search, the rendered header/title:
//   - includes "Outliner: Priority"
//   - omits the perspective label segments (no "‹ ... @ <name> ... ›")
//   - omits the sort suffix (no "Sort:")
//   - still surfaces the active filter badge and the search indicator
func TestCrossPriorityModeSuppressesPerspectiveAndSort(t *testing.T) {
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(160, 24)

	proj := &models.Project{ID: "proj-suppress", Name: "Suppress"}
	proj.Tasks = []*models.Task{makePriorityTask("t1", "Sample", intPtr(0))}
	proj.BuildTaskTree()
	m.loadedProjects = []*models.Project{proj}

	// Non-All perspective (Active = 2) plus a non-default sort that would
	// otherwise render a "Sort: priority" badge in the title suffix.
	m.activePerspective = 2
	m.sort = SortState{Field: SortByPriority, Ascending: false}

	// Active filter + search that must still render in Priority mode.
	m.filter.Priority = 1
	m.SetSearchQuery("alpha")

	m.enterCrossPriorityMode()

	title := ansi.Strip(m.renderOutlinerTitle(160))
	if !strings.Contains(title, "Outliner: Priority") {
		t.Errorf("title should advertise Priority mode, got %q", title)
	}
	if strings.Contains(title, "Sort:") {
		t.Errorf("Priority mode should suppress the sort suffix, got %q", title)
	}
	if !strings.Contains(title, "Filters: !Hi") {
		t.Errorf("active filter badge should still render, got %q", title)
	}

	tree := ansi.Strip(m.renderOutlinerTreeContent(160))
	if !strings.Contains(tree, "Outliner: Priority") {
		t.Errorf("tree content should advertise Priority mode, got %q", tree)
	}
	// Perspective label format from buildPerspectiveLabel:
	//   "‹ <prev>  @ <current>  <next> ›"
	// Any of these segments leaking in indicates the label was not suppressed.
	for _, leak := range []string{"@ Active", "‹ All", "Next ›"} {
		if strings.Contains(tree, leak) {
			t.Errorf("Priority mode should suppress perspective label segment %q, got %q", leak, tree)
		}
	}
	if !strings.Contains(tree, "Sample") && !strings.Contains(tree, "No tasks") {
		// Sanity: render produced *something* (rows or empty state).
		t.Errorf("tree render produced no recognizable content: %q", tree)
	}
	if !strings.Contains(tree, "Search: \"alpha\"") {
		t.Errorf("active search indicator should still render, got %q", tree)
	}
	if strings.Contains(tree, "Sort:") {
		t.Errorf("Priority mode should suppress the sort suffix in tree header, got %q", tree)
	}
}

// TestCrossPriorityModeStatusBarLabel verifies the status bar label swap from
// "Outliner" to "Outliner: Priority" while in priority mode.
func TestCrossPriorityModeStatusBarLabel(t *testing.T) {
	cfg := newTestConfig()
	m := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))

	off := m.crossStatusBarLabel()
	if !strings.HasPrefix(off, "Outliner · ") {
		t.Errorf("expected default status bar prefix, got %q", off)
	}
	if strings.Contains(off, "Priority") {
		t.Errorf("default status bar should not advertise Priority, got %q", off)
	}

	m.enterCrossPriorityMode()
	on := m.crossStatusBarLabel()
	if !strings.HasPrefix(on, "Outliner: Priority · ") {
		t.Errorf("expected status bar to advertise Priority mode, got %q", on)
	}
}

// TestCrossPriorityModeFingerprintInvalidatesMemoization verifies that
// flipping the priority mode (and toggling a collapsed group) changes the
// row-rebuild fingerprint so the memoization guard rebuilds rows.
func TestCrossPriorityModeFingerprintInvalidatesMemoization(t *testing.T) {
	m := minimalOutliner()
	base := m.crossProjectRowsFingerprint()

	m.crossPriorityMode = true
	if got := m.crossProjectRowsFingerprint(); got == base {
		t.Error("enabling priority mode should change rebuild fingerprint")
	}
	priorityNoCollapse := m.crossProjectRowsFingerprint()

	m.crossPriorityCollapsedGroups = map[PriorityGroup]bool{PriorityGroupWaitingOn: true}
	if got := m.crossProjectRowsFingerprint(); got == priorityNoCollapse {
		t.Error("collapsing a group should change rebuild fingerprint")
	}
}

// TestPriorityViewKeyTogglesCrossModeInOutliner verifies that the keymap
// `PriorityView` dep handler toggles cross-project Priority mode when the
// outliner is operating in cross-project context (m.project == nil) and applies
// the high-priority focus in place when a project is loaded.
func TestPriorityViewKeyTogglesCrossModeInOutliner(t *testing.T) {
	cfg := newTestConfig()
	pKey := tea.KeyPressMsg{Code: 'p', Text: "p"}

	// Cross-project context: `p` toggles the mode in place; no message emitted.
	cross := NewOutlinerModel(nil, nil, testRuntimeAt(cfg.DataDir(), cfg))
	model, cmd, ok := cross.Dispatcher.Dispatch(pKey)
	if !ok {
		t.Fatal("dispatcher should handle `p` in cross-project outliner")
	}
	if model != cross {
		t.Errorf("`p` should return the same OutlinerModel, got %T", model)
	}
	if cmd != nil {
		t.Errorf("`p` in cross-project context should not emit a tea.Cmd (toggles in place), got non-nil")
	}
	if !cross.crossPriorityMode {
		t.Error("`p` in cross-project outliner should enter Priority mode")
	}

	// Second press exits the mode.
	if _, _, _ = cross.Dispatcher.Dispatch(pKey); cross.crossPriorityMode {
		t.Error("second `p` should exit Priority mode")
	}

	// Single-project context: `p` applies the priority focus in place and does
	// not flip the unrelated crossPriorityMode flag.
	proj := &models.Project{ID: "p1", Name: "P1"}
	single := NewOutlinerModel(proj, nil, testRuntimeAt(cfg.DataDir(), cfg))
	_, singleCmd, ok := single.Dispatcher.Dispatch(pKey)
	if !ok {
		t.Fatal("dispatcher should handle `p` in single-project outliner")
	}
	if singleCmd != nil {
		t.Fatal("`p` in single-project context should apply the focus in place")
	}
	if single.smartFocus == nil || *single.smartFocus != OutlinerHighPriority {
		t.Fatalf("single-project focus = %v, want High Priority", single.smartFocus)
	}
	if single.sort.Field != SortByScore {
		t.Fatalf("single-project sort = %v, want Score", single.sort.Field)
	}
	if single.crossPriorityMode {
		t.Error("single-project `p` must not flip cross-project priority mode")
	}
}
