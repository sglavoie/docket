package tui

import (
	"strings"
	"testing"
)

// TestOutlinerModeCatalogHasUniqueIDs guards the stable identity contract: no two
// modes may share an ID.
func TestOutlinerModeCatalogHasUniqueIDs(t *testing.T) {
	seen := make(map[OutlinerModeID]bool)
	for _, mode := range OutlinerModeCatalog() {
		if seen[mode.ID] {
			t.Fatalf("duplicate mode ID %q", mode.ID)
		}
		seen[mode.ID] = true
		if mode.Label == "" {
			t.Errorf("mode %q has no label", mode.ID)
		}
	}
}

// TestOutlinerModeWrapsEveryPerspective proves each persisted perspective is
// wrapped by exactly one mode, so perspective meanings survive normalization.
func TestOutlinerModeWrapsEveryPerspective(t *testing.T) {
	perspectives := []int{
		PerspectiveAll, PerspectiveActive, PerspectiveNext, PerspectiveInbox,
		PerspectiveStandby, PerspectiveWaiting, PerspectiveSomeday,
		PerspectiveReview, PerspectiveSnoozed, PerspectiveCompleted,
	}
	for _, p := range perspectives {
		count := 0
		for _, mode := range OutlinerModeCatalog() {
			if mode.Perspective == p {
				count++
			}
		}
		if count != 1 {
			t.Errorf("perspective %d wrapped by %d modes, want exactly 1", p, count)
		}
		if _, ok := OutlinerModeForPerspective(p); !ok {
			t.Errorf("OutlinerModeForPerspective(%d) reported no mode", p)
		}
	}
}

// TestOutlinerModeWrapsEverySmartFocus proves each smart focus is wrapped.
func TestOutlinerModeWrapsEverySmartFocus(t *testing.T) {
	focuses := []OutlinerFocus{
		OutlinerDueSoon, OutlinerHighPriority, OutlinerBlocked,
		OutlinerParked, OutlinerPinned, OutlinerRecent,
	}
	for _, f := range focuses {
		mode, ok := OutlinerModeForSmartFocus(f)
		if !ok {
			t.Errorf("smart focus %s is not wrapped by any mode", f)
			continue
		}
		if mode.RowStrategy != RowStrategySmartFocus || mode.Category != ModeCategorySmart {
			t.Errorf("smart focus %s wrapped by non-smart mode %q", f, mode.ID)
		}
	}
}

// TestOutlinerModeWrapsLenses proves the priority and due-date lenses are wrapped.
func TestOutlinerModeWrapsLenses(t *testing.T) {
	priority, ok := OutlinerModeByID(ModePriorityLens)
	if !ok || !priority.PriorityLens {
		t.Fatalf("priority lens mode missing or not flagged: %+v", priority)
	}
	dueDate, ok := OutlinerModeByID(ModeDueDateLens)
	if !ok || dueDate.Lens == nil || *dueDate.Lens != OutlinerLensDueDate {
		t.Fatalf("due-date lens mode missing or mislinked: %+v", dueDate)
	}
}

// TestOutlinerModeManualSortOnlyForTreeOrder proves manual sort is active only
// for tree-order modes; every fixed-order horizon owns its ordering.
func TestOutlinerModeManualSortOnlyForTreeOrder(t *testing.T) {
	fixed := map[OutlinerModeID]bool{
		ModeToday: true, ModeNext: true, ModeBacklog: true,
		ModePriorityLens: true, ModeDueDateLens: true,
		ModeDailyReview: true, ModeActivity: true,
	}
	for _, mode := range OutlinerModeCatalog() {
		wantManual := !fixed[mode.ID]
		if mode.SupportsManualSort() != wantManual {
			t.Errorf("mode %q SupportsManualSort()=%v, want %v", mode.ID, mode.SupportsManualSort(), wantManual)
		}
		if mode.FixedOrder() == wantManual {
			t.Errorf("mode %q FixedOrder()=%v inconsistent with manual sort", mode.ID, mode.FixedOrder())
		}
	}
}

// TestOutlinerModeHorizonOrderPolicies pins the purpose-built horizon policies.
func TestOutlinerModeHorizonOrderPolicies(t *testing.T) {
	tests := []struct {
		id       OutlinerModeID
		policy   OutlinerOrderPolicy
		strategy OutlinerRowStrategy
	}{
		{ModeToday, OrderPolicyDailyPlan, RowStrategyToday},
		{ModeNext, OrderPolicyFocusQueue, RowStrategyNext},
		{ModeBacklog, OrderPolicyRanked, RowStrategyBacklog},
	}
	for _, tc := range tests {
		mode, ok := OutlinerModeByID(tc.id)
		if !ok {
			t.Fatalf("mode %q missing from catalog", tc.id)
		}
		if mode.OrderPolicy != tc.policy || mode.RowStrategy != tc.strategy {
			t.Errorf("mode %q = policy %d strategy %d, want %d/%d", tc.id, mode.OrderPolicy, mode.RowStrategy, tc.policy, tc.strategy)
		}
	}
}

// modeIDFromHandler executes a mode command's handler and returns the SetMode
// target it dispatches, failing the test if the command does not route through
// the canonical global Outliner via SetMode.
func modeIDFromHandler(t *testing.T, cmd *Command) OutlinerModeID {
	t.Helper()
	if cmd.Handler == nil {
		t.Fatalf("mode command %q has no handler", cmd.Name)
	}
	msg := cmd.Handler()
	show, ok := msg.(ShowGlobalOutlinerMsg)
	if !ok {
		t.Fatalf("mode command %q dispatched %T, want ShowGlobalOutlinerMsg", cmd.Name, msg)
	}
	if !show.Options.SetMode {
		t.Fatalf("mode command %q did not set SetMode", cmd.Name)
	}
	return show.Options.Mode
}

// isOutlinerModeCommand reports whether a palette command is a catalog-derived
// mode action (a SetMode dispatch into the global Outliner).
func isOutlinerModeCommand(cmd *Command) bool {
	if cmd.Handler == nil {
		return false
	}
	show, ok := cmd.Handler().(ShowGlobalOutlinerMsg)
	return ok && show.Options.SetMode
}

// TestEveryOutlinerModeIsRegisteredInCommandPalette proves the command palette
// derives one selectable action for every OutlinerModeCatalog entry — searchable
// by label, stable ID, and every alias — rather than a hand-maintained subset,
// that catalog additions appear without another registry edit, and that
// executing a representative mode from each category dispatches through SetMode
// into the canonical app-scoped Outliner.
func TestEveryOutlinerModeIsRegisteredInCommandPalette(t *testing.T) {
	catalog := OutlinerModeCatalog()

	// One derived palette action per catalog entry, indexed by the mode its
	// handler dispatches.
	byID := make(map[OutlinerModeID]Command)
	for _, item := range outlinerModePaletteItems() {
		if item.Category != ItemCategoryCommands || item.Command == nil {
			t.Fatalf("mode palette item %q is not a command item", item.Name)
		}
		id := modeIDFromHandler(t, item.Command)
		if _, dup := byID[id]; dup {
			t.Fatalf("mode %q has more than one palette action", id)
		}
		byID[id] = *item.Command
	}
	if len(byID) != len(catalog) {
		t.Fatalf("palette mode actions = %d, want one per catalog entry (%d)", len(byID), len(catalog))
	}

	for _, mode := range catalog {
		cmd, ok := byID[mode.ID]
		if !ok {
			t.Errorf("catalog mode %q has no palette action", mode.ID)
			continue
		}
		// Searchable by label (the command Name).
		if cmd.Name != mode.Label {
			t.Errorf("mode %q palette name = %q, want label %q", mode.ID, cmd.Name, mode.Label)
		}
		// Searchable by stable ID and every alias. The label collapses away since
		// the command Name already covers it.
		searchable := map[string]bool{strings.ToLower(cmd.Name): true}
		for _, a := range cmd.Aliases {
			searchable[strings.ToLower(a)] = true
		}
		if !searchable[strings.ToLower(string(mode.ID))] {
			t.Errorf("mode %q not searchable by its stable ID", mode.ID)
		}
		for _, alias := range mode.Aliases {
			if !searchable[strings.ToLower(alias)] {
				t.Errorf("mode %q not searchable by alias %q", mode.ID, alias)
			}
		}
	}

	// Catalog additions appear in the live palette without another registry edit:
	// loadAllItems surfaces every mode even with an empty command registry.
	rt := newTestConfig()
	palette := NewCommandPaletteModel(testRuntimeAt(rt.DataDir(), rt), NewCommandRegistry(), ViewCrossProjectOutliner, nil)
	present := make(map[OutlinerModeID]bool)
	for _, item := range palette.loadAllItems() {
		if item.Category == ItemCategoryCommands && item.Command != nil && isOutlinerModeCommand(item.Command) {
			present[modeIDFromHandler(t, item.Command)] = true
		}
	}
	for _, mode := range catalog {
		if !present[mode.ID] {
			t.Errorf("mode %q is missing from the loaded command palette items", mode.ID)
		}
	}

	// Execute a representative mode from each category; each must dispatch its own
	// SetMode into the global Outliner.
	representatives := []OutlinerModeID{
		ModeToday,        // Work
		ModeInbox,        // Workflow
		ModeHighPriority, // Smart
		ModeDueDateLens,  // Lens
		ModeDailyReview,  // Review
	}
	for _, id := range representatives {
		cmd, ok := byID[id]
		if !ok {
			t.Fatalf("representative mode %q missing from palette", id)
		}
		if got := modeIDFromHandler(t, &cmd); got != id {
			t.Errorf("representative mode %q handler dispatched mode %q", id, got)
		}
	}
}
