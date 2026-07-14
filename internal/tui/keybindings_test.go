package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewKeybindingRegistry(t *testing.T) {
	r := NewKeybindingRegistry()
	if r == nil {
		t.Fatal("NewKeybindingRegistry returned nil")
	}
	if len(r.bindings) == 0 {
		t.Fatal("Registry has no bindings")
	}
}

func hasHelpItem(items []HelpItem, key, description string) bool {
	for _, item := range items {
		if item.Key == key && item.Description == description {
			return true
		}
	}
	return false
}

func countHelpItem(items []HelpItem, key, description string) int {
	count := 0
	for _, item := range items {
		if item.Key == key && item.Description == description {
			count++
		}
	}
	return count
}

func assertHasHelpItem(t *testing.T, items []HelpItem, view ViewType, key, description string) {
	t.Helper()
	if !hasHelpItem(items, key, description) {
		t.Fatalf("%s help missing %s %q binding", view, key, description)
	}
}

func assertLacksHelpItem(t *testing.T, items []HelpItem, view ViewType, key, description string) {
	t.Helper()
	if hasHelpItem(items, key, description) {
		t.Fatalf("%s help should not advertise %s %q binding", view, key, description)
	}
}

func TestGetForView_Dashboard(t *testing.T) {
	// Dashboard is migrated to declarative keymap; verify via buildKeyMap().
	m := &DashboardModel{}
	keys := m.buildKeyMap().HandledKeys()
	if len(keys) == 0 {
		t.Fatal("No keys returned from DashboardModel.buildKeyMap()")
	}
	foundJK := false
	foundEnter := false
	for _, k := range keys {
		if k == "j" || k == "k" {
			foundJK = true
		}
		if k == "enter" {
			foundEnter = true
		}
	}
	if !foundJK {
		t.Error("Expected j/k navigation keys in Dashboard keymap")
	}
	if !foundEnter {
		t.Error("Expected enter key in Dashboard keymap")
	}
	for _, key := range []string{"ctrl+u", "ctrl+d", "ctrl+b", "ctrl+f"} {
		if !slices.Contains(keys, key) {
			t.Errorf("Expected %s navigation key in Dashboard keymap", key)
		}
	}
}

func TestGetForView_KeybindingExplorerMetadata(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewKeybindingExplorer)

	assertHasHelpItem(t, items, ViewKeybindingExplorer, "p", "Toggle by prefix/category")
	for _, key := range []string{"n", "N"} {
		for _, item := range items {
			if item.Key == key {
				t.Fatalf("Keybinding Explorer should not advertise stale %s binding: %+v", key, item)
			}
		}
	}
}

func TestGetForView_ScopedTaskGlobalsStayOutOfDashboardHelp(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewDashboard)

	taskOnlyBindings := []HelpItem{
		{Key: "space", Description: "Toggle task detail peek (toggles selection in multi-select)"},
		{Key: "q", Description: "Toggle queue"},
		{Key: "w", Description: "Toggle today's plan"},
		{Key: "ctrl+o", Description: "Open task reference copy"},
		{Key: "ctrl+n", Description: "Open project Notes"},
		{Key: ".", Description: "Repeat last action"},
		{Key: "y", Description: "Copy title"},
		{Key: "Y", Description: "Copy description"},
		{Key: "ctrl+Y", Description: "Copy title and description"},
	}
	for _, binding := range taskOnlyBindings {
		assertLacksHelpItem(t, items, ViewDashboard, binding.Key, binding.Description)
	}

	globalBindings := []HelpItem{
		{Key: "ctrl+c", Description: "Quit"},
		{Key: "?", Description: "Toggle help"},
		{Key: `ctrl+\`, Description: "Keybinding Explorer"},
		{Key: ":", Description: "Command palette"},
		{Key: "ctrl+,", Description: "Open Settings"},
	}
	for _, binding := range globalBindings {
		assertHasHelpItem(t, items, ViewDashboard, binding.Key, binding.Description)
	}
}

func TestGetForView_TaskCapableViewsIncludeScopedTaskGlobals(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewOutliner)

	scopedBindings := []HelpItem{
		{Key: "space", Description: "Toggle task detail peek (toggles selection in multi-select)"},
		{Key: "q", Description: "Toggle queue"},
		{Key: "w", Description: "Toggle today's plan"},
		{Key: "ctrl+o", Description: "Open task reference copy"},
		{Key: "ctrl+n", Description: "Open project Notes"},
		{Key: ".", Description: "Repeat last action"},
		{Key: "y", Description: "Copy title"},
		{Key: "Y", Description: "Copy description"},
		{Key: "ctrl+Y", Description: "Copy title and description"},
	}
	for _, binding := range scopedBindings {
		assertHasHelpItem(t, items, ViewOutliner, binding.Key, binding.Description)
	}
	assertLacksHelpItem(t, items, ViewOutliner, "w", "Add to today's plan")
}

func TestActualTaskViewHelpIncludesScopedAppBindings(t *testing.T) {
	app, project := newTestAppModelWithProject()

	tree := NewProjectTreeModel(project, app.configManager, app.runtime)
	treeItems := tree.GetHelpItems()
	assertHasHelpItem(t, treeItems, ViewProjectTree, "w", "Toggle today's plan")
	assertHasHelpItem(t, treeItems, ViewProjectTree, "ctrl+o", "Open task reference copy")
	assertHasHelpItem(t, treeItems, ViewProjectTree, "ctrl+n", "Open project Notes")
	assertLacksHelpItem(t, treeItems, ViewProjectTree, "w", "Add to today's plan")
	if count := countHelpItem(treeItems, "ctrl+n", "Open project Notes"); count != 1 {
		t.Fatalf("ProjectTree help has %d ctrl+n Open project Notes rows, want 1", count)
	}

	allTasksItems := (&AllTasksViewModel{}).GetHelpItems()
	assertHasHelpItem(t, allTasksItems, ViewAllTasks, "space", "Toggle task detail peek (toggles selection in multi-select)")
	assertHasHelpItem(t, allTasksItems, ViewAllTasks, "w", "Toggle today's plan")
	assertHasHelpItem(t, allTasksItems, ViewAllTasks, "y", "Copy title")

	detail := NewTaskDetailModel(project.Tasks[0], project, app.runtime)
	detailItems := detail.GetHelpItems()
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "j/k", "Scroll down/up")
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "q", "Toggle queue")
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "w", "Toggle today's plan")
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "ctrl+o", "Open task reference copy")
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "ctrl+n", "Open project Notes")
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "y", "Copy title")
	assertLacksHelpItem(t, detailItems, ViewTaskDetail, "space", "Toggle task detail peek (toggles selection in multi-select)")
}

func TestFocusBoardSimpleKeyGrammarHandledKeysAndHelp(t *testing.T) {
	m := &FocusBoardModel{}
	keys := m.HandledKeys()

	for _, key := range []string{
		"a", "x", "p", "P", "/", "?", "enter",
		"tab", "shift+tab", "shift+left", "shift+right",
		"j", "down", "k", "up", "G", "ctrl+u", "ctrl+d",
		"g", "gg", "gp", "n", "N",
		"f", "fp",
		"F", "F#", "Fh", "Fp", "Fi", "Fx", "Ft", "Fs", "Fd", "Fa", "FS",
		"1", "2", "3", "4", "5", "0",
		",", ",d", "!", "!1", "d", "dd", "z", "zd", "m", "mn", "c", "cb", "t", "tn", "tN", "tD",
		"b", "bb", "o", "D", "e", "S", "O", "ctrl+b", "ctrl+.",
	} {
		if !slices.Contains(keys, key) {
			t.Errorf("FocusBoard HandledKeys missing canonical key %q", key)
		}
	}

	for _, key := range []string{
		"q", "w", "X", "E", "v", "space", "A", "J", "K",
		"ctrl+f", "ctrl+r", "ctrl+t", "r",
		"fa",
		"P0",
		".",
	} {
		if slices.Contains(keys, key) {
			t.Errorf("FocusBoard HandledKeys should not preserve removed key %q", key)
		}
	}

	items := m.GetHelpItems()
	for _, want := range []HelpItem{
		{Key: "a", Description: "Quick-add board item"},
		{Key: "x", Description: "Complete or dismiss item"},
		{Key: "p", Description: "Plan or set priority"},
		{Key: "P", Description: "Park task"},
		{Key: "/", Description: "Search"},
		{Key: "?", Description: "Toggle help"},
		{Key: "enter", Description: "View task details"},
		{Key: "gg", Description: "Jump to top"},
		{Key: "gp", Description: "Go to task in project"},
		{Key: "n", Description: "Go to next parent task"},
		{Key: "N", Description: "Go to previous parent task"},
		{Key: "o", Description: "View task details"},
		{Key: "D", Description: "Mark done"},
		{Key: "e", Description: "Edit task"},
		{Key: "S", Description: "Edit score dimensions"},
		{Key: "O", Description: "Open URL in browser"},
		{Key: "ctrl+b", Description: "Move task to project"},
		{Key: "ctrl+.", Description: "Quick note on task"},
		{Key: "1", Description: "Sort within groups: priority"},
		{Key: "2", Description: "Sort within groups: due date"},
		{Key: "3", Description: "Sort within groups: created"},
		{Key: "4", Description: "Sort within groups: status"},
		{Key: "5", Description: "Sort within groups: score"},
		{Key: "0", Description: "Reset group sort"},
		{Key: "fp", Description: "Toggle inline preview"},
		{Key: ":", Description: "Command palette"},
		{Key: "esc", Description: "Back / close"},
		{Key: "u", Description: "Undo last task action"},
		{Key: "Fh", Description: "Toggle hide-dimmed filter"},
	} {
		assertHasHelpItem(t, items, ViewFocusBoard, want.Key, want.Description)
	}
	for _, key := range []string{
		"q", "w", "X", "E", "v", "space", "A", "J", "K",
		"ctrl+f", "ctrl+r", "ctrl+t", "r",
		"g", "f", "fa",
		"P0",
		",", ",d", "!", "!1", "d", "z", "m", "c", "t", ".", "b",
		"ctrl+o", "ctrl+n",
		"y", "Y", "ctrl+Y",
	} {
		for _, item := range items {
			if item.Key == key {
				t.Errorf("FocusBoard help should not advertise removed key %q: %+v", key, item)
			}
		}
	}
}

func TestGetForView_FocusBoardIncludesTodayUngatedParity(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewFocusBoard)

	for _, want := range []HelpItem{
		{Key: "enter", Description: "View task details"},
		{Key: "o", Description: "View task details"},
		{Key: "gg", Description: "Jump to top"},
		{Key: "gp", Description: "Go to task in project"},
		{Key: "n", Description: "Go to next parent task"},
		{Key: "N", Description: "Go to previous parent task"},
		{Key: "1", Description: "Sort within groups: priority"},
		{Key: "2", Description: "Sort within groups: due date"},
		{Key: "3", Description: "Sort within groups: created"},
		{Key: "4", Description: "Sort within groups: status"},
		{Key: "5", Description: "Sort within groups: score"},
		{Key: "0", Description: "Reset group sort"},
		{Key: "fp", Description: "Toggle inline preview"},
		{Key: "ctrl+b", Description: "Move task to project"},
		{Key: "ctrl+.", Description: "Quick note on task"},
	} {
		assertHasHelpItem(t, items, ViewFocusBoard, want.Key, want.Description)
	}

	for _, removed := range []HelpItem{
		{Key: "w", Description: "Toggle today's plan"},
		{Key: "ctrl+o", Description: "Open task reference copy"},
		{Key: "ctrl+n", Description: "Open project Notes"},
		{Key: "space", Description: "Toggle task detail peek (toggles selection in multi-select)"},
	} {
		assertLacksHelpItem(t, items, ViewFocusBoard, removed.Key, removed.Description)
	}
}

func TestFocusBoardHelpIncludesSelectedTaskParity(t *testing.T) {
	m, _, _ := focusBoardTaskActionFixture(t)
	items := m.GetHelpItems()

	for _, want := range []HelpItem{
		{Key: "tn", Description: "New task (no template)"},
		{Key: "tN", Description: "New task (with template picker)"},
		{Key: "tD", Description: "Delete task"},
		{Key: ",d", Description: "Mark done"},
		{Key: "ctrl+b", Description: "Move task to project"},
		{Key: "ctrl+.", Description: "Quick note on task"},
	} {
		assertHasHelpItem(t, items, ViewFocusBoard, want.Key, want.Description)
	}
}

func TestFocusBoardHelpDistinguishesParentNavigationFromSnoozeNextMonday(t *testing.T) {
	m, _, _ := focusBoardTaskActionFixture(t)
	items := m.GetHelpItems()

	assertHasHelpItem(t, items, ViewFocusBoard, "n", "Go to next parent task")
	assertHasHelpItem(t, items, ViewFocusBoard, "N", "Go to previous parent task")
	assertHasHelpItem(t, items, ViewFocusBoard, "zn", "Snooze until next Monday")
	assertLacksHelpItem(t, items, ViewFocusBoard, "n", "Snooze until next Monday")
	assertLacksHelpItem(t, items, ViewFocusBoard, "zn", "Go to next parent task")
}

func TestFocusBoardSimpleKeyGrammarRemovedAppAliasesFallThrough(t *testing.T) {
	app, project := newTestAppModelWithProject()
	view := &ctrlORecorderTaskRefView{task: project.Tasks[0], project: project}
	app.views.Register(ViewFocusBoard, view)
	app.viewStack = []ViewType{ViewDashboard, ViewFocusBoard}
	app.focusQueueIDs = map[string]bool{}
	app.dailyPlanIDs = map[string]bool{}

	removed := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"queue", syntheticKey('q')},
		{"plan", syntheticKey('w')},
		{"redo", syntheticCtrlKey('r')},
		{"reference", syntheticCtrlKey('o')},
		{"notes", syntheticCtrlKey('n')},
		{"peek", syntheticSpecialKey(tea.KeySpace)},
		{"copy title", syntheticKey('y')},
		{"copy description", syntheticKey('Y')},
		{"copy title and description", syntheticCtrlKey('y')},
	}
	for _, tc := range removed {
		t.Run(tc.name, func(t *testing.T) {
			if handled, _ := app.handleGlobalKeyPress(tc.key); handled {
				t.Fatalf("FocusBoard should let removed app-level alias %q fall through", tc.key.String())
			}
		})
	}

	if handled, _ := app.handleGlobalKeyPress(syntheticKey('u')); !handled {
		t.Fatal("FocusBoard should keep canonical app-level undo key u handled")
	}
}

func TestGetGlobalBindings_IncludesHandledSemicolonShortcuts(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetGlobalBindings()

	globalSemicolonBindings := []HelpItem{
		{Key: ";!", Description: "Global Priority Update"},
		{Key: ";td", Description: "Global Due Date Update"},
		{Key: ";tm", Description: "Global Task Move"},
		{Key: ";ts", Description: "Global Section Change"},
		{Key: ";tN", Description: "Global Template Task Create"},
	}
	for _, binding := range globalSemicolonBindings {
		assertHasHelpItem(t, items, ViewDashboard, binding.Key, binding.Description)
	}
}

func TestGetForView_ProjectTree(t *testing.T) {
	// ProjectTree is migrated to declarative keymap; verify via buildKeyMap().
	m := &ProjectTreeModel{}
	keys := m.buildKeyMap().HandledKeys()
	if len(keys) == 0 {
		t.Fatal("No keys returned from ProjectTreeModel.buildKeyMap()")
	}

	foundFold := false
	foundStatus := false
	for _, k := range keys {
		if k == "fa" || k == "fo" || k == "fc" {
			foundFold = true
		}
		if k == ",s" {
			foundStatus = true
		}
	}
	if !foundFold {
		t.Error("Expected fold keys (fa/fo/fc) in ProjectTree keymap")
	}
	if !foundStatus {
		t.Error("Expected status change key ',s' in ProjectTree keymap")
	}
	for _, key := range []string{"fs", "fS"} {
		if slices.Contains(keys, key) {
			t.Errorf("ProjectTree keymap should not expose section fold key %s", key)
		}
	}

	r := NewKeybindingRegistry()
	for _, item := range r.GetForView(ViewProjectTree) {
		if item.Key == "fs" || item.Key == "fS" {
			t.Fatalf("ProjectTree registry should not advertise section fold key %s: %+v", item.Key, item)
		}
	}
}

func TestGetForView_OutlinerSectionFold(t *testing.T) {
	keys := (&OutlinerModel{}).HandledKeys()
	for _, key := range []string{"fs", "fS"} {
		if !slices.Contains(keys, key) {
			t.Fatalf("Expected OutlinerModel.HandledKeys to include section fold key %s", key)
		}
	}

	r := NewKeybindingRegistry()
	for _, view := range []ViewType{ViewOutliner, ViewCrossProjectOutliner} {
		found := map[string]bool{
			"fs": false,
			"fS": false,
		}
		for _, item := range r.GetForView(view) {
			if item.Key == "fs" && item.Description == "Toggle section fold" {
				found["fs"] = true
			}
			if item.Key == "fS" && item.Description == "Toggle all section folds" {
				found["fS"] = true
			}
		}
		for key, ok := range found {
			if !ok {
				t.Fatalf("%s registry help missing %s section fold binding", view, key)
			}
		}
	}
}

func TestGetForView_OutlinerTagManagerScope(t *testing.T) {
	r := NewKeybindingRegistry()

	assertHelpItem := func(view ViewType, key, desc string) {
		t.Helper()
		for _, item := range r.GetForView(view) {
			if item.Key == key && item.Description == desc {
				return
			}
			if item.Key == key && (item.Description == "Tag manager" || item.Description == "Global tag manager") {
				t.Fatalf("%s ctrl+t description = %q, want %q", view, item.Description, desc)
			}
		}
		t.Fatalf("%s help missing %s %q binding", view, key, desc)
	}

	assertHelpItem(ViewOutliner, "ctrl+t", "Tag manager")
	assertHelpItem(ViewCrossProjectOutliner, "ctrl+t", "Global tag manager")
}

func TestGetForView_OutlinerSectionManagerScope(t *testing.T) {
	r := NewKeybindingRegistry()

	for _, view := range []ViewType{ViewOutliner, ViewCrossProjectOutliner} {
		found := false
		for _, item := range r.GetForView(view) {
			if item.Key == "ctrl+s" && item.Description == "Section manager" {
				found = true
			}
			if item.Key == "ctrl+s" && item.Description != "Section manager" {
				t.Fatalf("%s ctrl+s description = %q, want Section manager", view, item.Description)
			}
		}
		if !found {
			t.Fatalf("%s help missing ctrl+s Section manager binding", view)
		}
	}
}

func TestGetForView_ProjectTreeAndOutlinerUseCtrlNForProjectNotes(t *testing.T) {
	r := NewKeybindingRegistry()

	for _, view := range []ViewType{ViewProjectTree, ViewOutliner} {
		items := r.GetForView(view)
		foundCtrlN := false
		for _, item := range items {
			if item.Key == "ctrl+n" && item.Description == "Open project Notes" {
				foundCtrlN = true
			}
			if item.Key == "ctrl+o" && item.Description == "Open outliner view" {
				t.Fatalf("%s should not advertise ctrl+o as Outliner", view)
			}
			if item.Key == "{" && item.Description == "Open project Notes" {
				t.Fatalf("%s should not advertise hidden Notes alias {", view)
			}
		}
		if !foundCtrlN {
			t.Fatalf("%s missing ctrl+n Open project Notes binding", view)
		}
	}
}

func TestGetForView_ProjectNotesScopeDoesNotAdvertiseStaleCtrlN(t *testing.T) {
	r := NewKeybindingRegistry()

	for _, view := range projectNotesViews() {
		items := r.GetForView(view)
		for _, item := range items {
			if item.Key == "ctrl+n" && item.Description != "Open project Notes" {
				t.Fatalf("%s advertises ctrl+n as %q", view, item.Description)
			}
		}
	}
}

func TestGetForView_WeeklyReviewAdvertisesScopedProjectNotesCtrlN(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewWeeklyReview)

	foundProjectNotes := false
	for _, item := range items {
		if item.Key != "ctrl+n" {
			continue
		}
		if item.Description == "Open project Notes" {
			foundProjectNotes = true
			continue
		}
		if item.Description == "Next unreviewed section" {
			t.Fatalf("Weekly Review should not advertise hidden local ctrl+n binding %q", item.Description)
		}
		t.Fatalf("Weekly Review advertises unexpected ctrl+n binding %q", item.Description)
	}
	if !foundProjectNotes {
		t.Fatal("Weekly Review should advertise scoped ctrl+n Open project Notes binding")
	}

	keys := (&WeeklyReviewModel{}).buildKeyMap().HandledKeys()
	if !slices.Contains(keys, "ctrl+n") {
		t.Fatal("Weekly Review should keep local ctrl+n handled as a hidden fallback")
	}
}

func TestListNavigationGroupConsumersExposeArrowKeys(t *testing.T) {
	tests := []struct {
		name string
		keys []string
	}{
		{name: "All Tasks", keys: (&AllTasksViewModel{}).HandledKeys()},
		{name: "Focus", keys: (&FocusViewModel{}).HandledKeys()},
		{name: "Priority", keys: (&PriorityViewModel{}).HandledKeys()},
		{name: "Recent Activity", keys: (&RecentActivityModel{}).HandledKeys()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"j", "down", "k", "up"} {
				if !slices.Contains(tc.keys, key) {
					t.Errorf("HandledKeys missing %q", key)
				}
			}
		})
	}
}

func TestGetForView_InboxProcessorPriority(t *testing.T) {
	// InboxProcessor is migrated to declarative keymap; verify via buildKeyMap().
	m := &InboxProcessorModel{}
	keys := m.buildKeyMap().HandledKeys()
	if len(keys) == 0 {
		t.Fatal("No keys returned from InboxProcessorModel.buildKeyMap()")
	}
	// Priority keys 1-5 should be handled.
	found1 := slices.Contains(keys, "1")
	if !found1 {
		t.Error("Expected priority key '1' in InboxProcessor keymap")
	}
}

func TestGetForView_InboxProcessor(t *testing.T) {
	// InboxProcessor is migrated to declarative keymap; verify via buildKeyMap().
	m := &InboxProcessorModel{}
	keys := m.buildKeyMap().HandledKeys()
	if len(keys) == 0 {
		t.Fatal("No keys returned from InboxProcessorModel.buildKeyMap()")
	}
	// 'n' is the skip/next key in the inbox processor.
	foundN := slices.Contains(keys, "n")
	if !foundN {
		t.Error("Expected 'n' key in InboxProcessor keymap")
	}
}

// TestSharedTriagePrimaryVocabulary verifies both the local and global inbox
// processors expose the canonical primary triage actions through the shared
// TriagePrimaryGroup keymap helper. The vocabulary must stay identical across
// both processors so help, palette labels, and tutor content describe one model.
func TestSharedTriagePrimaryVocabulary(t *testing.T) {
	primaryKeys := []string{"n", "e", "D", "T", "u", "?"}
	cases := []struct {
		name string
		keys []string
	}{
		{"InboxProcessor", (&InboxProcessorModel{}).buildKeyMap().HandledKeys()},
		{"GlobalInboxProcessor", (&GlobalInboxProcessorModel{}).buildKeyMap().HandledKeys()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range primaryKeys {
				if !slices.Contains(tc.keys, k) {
					t.Errorf("%s: missing primary triage key %q", tc.name, k)
				}
			}
			// ctrl+t (legacy tags chord) must not appear; T is the canonical key.
			if slices.Contains(tc.keys, "ctrl+t") {
				t.Errorf("%s: legacy chord 'ctrl+t' must be removed in favor of 'T'", tc.name)
			}
		})
	}
}

func TestGetForView_IncludesGlobalBindings(t *testing.T) {
	r := NewKeybindingRegistry()

	for _, view := range []ViewType{ViewDashboard, ViewProjectTree, ViewPriorityView, ViewInboxProcessor} {
		items := r.GetForView(view)
		foundQuit := false
		foundHelp := false
		for _, item := range items {
			if item.Key == "ctrl+c" && item.Description == "Quit" {
				foundQuit = true
			}
			if item.Key == "?" && item.Description == "Toggle help" {
				foundHelp = true
			}
		}
		if !foundQuit {
			t.Errorf("Global quit binding missing for view %v", view)
		}
		if !foundHelp {
			t.Errorf("Global help binding missing for view %v", view)
		}
	}
}

func TestGetGlobalBindings(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetGlobalBindings()

	if len(items) == 0 {
		t.Fatal("No global bindings returned")
	}

	foundCommandPalette := false
	foundDashboard := false
	for _, item := range items {
		if item.Key == ":" && item.Description == "Command palette" {
			foundCommandPalette = true
		}
		if item.Key == "ctrl+esc" && item.Description == "Go to Dashboard" {
			foundDashboard = true
		}
	}
	if !foundCommandPalette {
		t.Error("Expected command palette in global bindings")
	}
	if !foundDashboard {
		t.Error("Expected Ctrl+Esc dashboard jump in global bindings")
	}
	removedShortcut := "ctrl+" + "Z"
	removedDescription := "Undo/redo" + " stack"
	if hasHelpItem(items, removedShortcut, removedDescription) {
		t.Error("Global bindings should not advertise removed undo history shortcut")
	}

	for _, item := range items {
		if item.Category == "Folds" || item.Category == "Task Operations" {
			t.Errorf("Global bindings should not include view-specific category %s", item.Category)
		}
	}
}

func TestContainsView(t *testing.T) {
	tests := []struct {
		views    []ViewType
		target   ViewType
		expected bool
	}{
		{[]ViewType{ViewDashboard, ViewProjectTree}, ViewDashboard, true},
		{[]ViewType{ViewDashboard, ViewProjectTree}, ViewPriorityView, false},
		{[]ViewType{}, ViewDashboard, false},
		{[]ViewType{ViewProjectTree}, ViewProjectTree, true},
	}

	for _, tc := range tests {
		result := containsView(tc.views, tc.target)
		if result != tc.expected {
			t.Errorf("containsView(%v, %v) = %v, want %v", tc.views, tc.target, result, tc.expected)
		}
	}
}

func TestGetAllDeduplicatedPreservesScope(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetAllDeduplicated()

	// Global bindings must have nil Views.
	for _, item := range items {
		if item.Key == "ctrl+c" && item.Description == "Quit" {
			if item.Views != nil {
				t.Error("Global binding 'ctrl+c' should have nil Views")
			}
			break
		}
	}

	// View-specific binding with a single view.
	for _, item := range items {
		if item.Key == "Enter" && item.Description == "Select project" {
			if len(item.Views) == 0 {
				t.Error("Dashboard-only 'Enter' binding should have non-nil Views")
			}
			if !containsView(item.Views, ViewDashboard) {
				t.Error("Dashboard-only 'Enter' binding should include ViewDashboard")
			}
			break
		}
	}

	// Status bindings are shared across registry views; verify non-global scope.
	// Note: views migrated to declarative keymaps no longer appear in the registry,
	// so the count reflects only the remaining registry-based views (Outliner, ProjectTree).
	for _, item := range items {
		if item.Key == ",s" && item.Description == "Start (in progress)" {
			if len(item.Views) < 2 {
				t.Errorf(",s binding should span at least 2 views, got %d", len(item.Views))
			}
			if item.Views == nil {
				t.Error(",s binding should not be global")
			}
			break
		}
	}

	// Every deduplicated item must have Views set (non-nil) for view-specific,
	// or nil for global. Verify no empty-but-non-nil slices exist.
	for _, item := range items {
		if item.Views != nil && len(item.Views) == 0 {
			t.Errorf("binding %q (%q) has empty non-nil Views slice", item.Key, item.Description)
		}
	}
}

func TestScopeText(t *testing.T) {
	tests := []struct {
		views []ViewType
		want  string
	}{
		{nil, "Global"},
		{[]ViewType{}, "Global"},
		{[]ViewType{ViewDashboard}, "Dashboard"},
		{[]ViewType{ViewDashboard, ViewProjectTree}, "Dashboard, ProjectTree"},
		{[]ViewType{ViewDashboard, ViewProjectTree, ViewPriorityView}, "Dashboard, ProjectTree, PriorityView"},
		{[]ViewType{ViewDashboard, ViewProjectTree, ViewPriorityView, ViewFocusView}, "4 views"},
		{[]ViewType{ViewDashboard, ViewProjectTree, ViewPriorityView, ViewFocusView, ViewOutliner}, "5 views"},
	}
	for _, tc := range tests {
		got := scopeText(tc.views)
		if got != tc.want {
			t.Errorf("scopeText(%v) = %q, want %q", tc.views, got, tc.want)
		}
	}
}

func TestKeybindingCategories(t *testing.T) {
	// ProjectTree is migrated to declarative keymap; verify via buildKeyMap().
	m := &ProjectTreeModel{}
	keys := m.buildKeyMap().HandledKeys()
	if len(keys) == 0 {
		t.Fatal("No keys from ProjectTreeModel.buildKeyMap()")
	}

	// Spot-check that the keymap includes keys from the expected categories.
	hasNav := false
	hasFold := false
	hasStatus := false
	hasFilter := false
	for _, k := range keys {
		switch k {
		case "j", "k", "G":
			hasNav = true
		case "fa", "fo":
			hasFold = true
		case ",s", ",d":
			hasStatus = true
		case "F", "Fp", "Fx":
			hasFilter = true
		}
	}
	if !hasNav {
		t.Error("Expected navigation keys in ProjectTree keymap")
	}
	if !hasFold {
		t.Error("Expected fold keys in ProjectTree keymap")
	}
	if !hasStatus {
		t.Error("Expected status keys in ProjectTree keymap")
	}
	if !hasFilter {
		t.Error("Expected filter keys in ProjectTree keymap")
	}
	for _, key := range []string{"fs", "fS"} {
		if slices.Contains(keys, key) {
			t.Errorf("ProjectTree keymap should not categorize unsupported section fold key %s", key)
		}
	}
}
