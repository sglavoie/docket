package tui

import (
	"slices"
	"strings"
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

func TestProjectViewToggle_KeybindingRegistryDescriptions(t *testing.T) {
	registry := NewKeybindingRegistry()
	// ProjectTree was retired: the single-project outliner has no second view to
	// switch to, so it no longer advertises any ctrl+p binding. ctrl+p survives
	// only on the cross-project outliner as a meaningful drill-down.
	const crossDescription = "Open highlighted project's outliner"

	for _, item := range registry.GetForView(ViewOutliner) {
		if item.Key == "ctrl+p" {
			t.Fatalf("single-project outliner should not advertise a ctrl+p binding, got %q", item.Description)
		}
	}
	assertHasHelpItem(t, registry.GetForView(ViewCrossProjectOutliner), ViewCrossProjectOutliner, "ctrl+p", crossDescription)
	assertLacksHelpItem(t, registry.GetForView(ViewCrossProjectOutliner), ViewCrossProjectOutliner, "ctrl+p", "Toggle project tree/outliner")

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

func assertHasHelpItem(t *testing.T, items []HelpItem, view PresentationID, key, description string) {
	t.Helper()
	if !hasHelpItem(items, key, description) {
		t.Fatalf("%s help missing %s %q binding", view, key, description)
	}
}

func assertLacksHelpItem(t *testing.T, items []HelpItem, view PresentationID, key, description string) {
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
	for _, key := range []string{"ctrl+u", "ctrl+d", "h", "l", "a", "e", "v", "/", "shift+enter"} {
		if !slices.Contains(keys, key) {
			t.Errorf("Expected %s navigation key in Dashboard keymap", key)
		}
	}
}

func TestGetForView_ScopedTaskGlobalsStayOutOfDashboardHelp(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewDashboard)

	taskOnlyBindings := []HelpItem{
		{Key: "enter", Description: "Open task detail"},
		{Key: "shift+enter", Description: "Peek task detail"},
		{Key: "q", Description: "Toggle queue"},
		{Key: "w", Description: "Toggle today's plan"},
		{Key: ".", Description: "Repeat last action"},
		{Key: "y", Description: "Copy title"},
		{Key: "Y", Description: "Copy description"},
		{Key: "ctrl+y", Description: "Copy title and description"},
	}
	for _, binding := range taskOnlyBindings {
		assertLacksHelpItem(t, items, ViewDashboard, binding.Key, binding.Description)
	}

	globalBindings := []HelpItem{
		{Key: "ctrl+c", Description: "Quit"},
		{Key: "?", Description: "Help"},
		{Key: ":", Description: "Command palette"},
		{Key: "ctrl+g", Description: "Go to destination"},
		{Key: "ctrl+n", Description: "Task composer"},
	}
	for _, binding := range globalBindings {
		assertHasHelpItem(t, items, ViewDashboard, binding.Key, binding.Description)
	}
}

func TestGetForView_TaskCapableViewsIncludeScopedTaskGlobals(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewOutliner)

	scopedBindings := []HelpItem{
		{Key: "enter", Description: "Open task detail"},
		{Key: "shift+enter", Description: "Peek task detail"},
		{Key: "q", Description: "Toggle queue"},
		{Key: "w", Description: "Toggle today's plan"},
		{Key: "ctrl+n", Description: "Task composer"},
		{Key: ".", Description: "Repeat last action"},
		{Key: "y", Description: "Copy title"},
		{Key: "Y", Description: "Copy description"},
		{Key: "ctrl+y", Description: "Copy title and description"},
	}
	for _, binding := range scopedBindings {
		assertHasHelpItem(t, items, ViewOutliner, binding.Key, binding.Description)
	}
	assertLacksHelpItem(t, items, ViewOutliner, "w", "Add to today's plan")
}

func TestQueueToggleIsAppLevelForEveryTaskReferenceView(t *testing.T) {
	r := NewKeybindingRegistry()
	for _, view := range taskReferenceViews() {
		assertHasHelpItem(t, r.GetAppLevelForView(view), view, "q", "Toggle queue")
	}

	for _, binding := range r.bindings {
		if binding.Key == "q" && binding.Description == "Toggle queue" && !binding.AppLevel {
			t.Fatalf("queue toggle must be handled only by the app-level dispatcher: %+v", binding)
		}
	}
}

func TestActualTaskViewHelpIncludesScopedAppBindings(t *testing.T) {
	app, project := newTestAppModelWithProject()

	outliner := NewOutlinerModel(project, app.configManager, app.runtime)
	outlinerItems := outliner.GetHelpItems()
	assertHasHelpItem(t, outlinerItems, ViewOutliner, "w", "Toggle today's plan")
	assertHasHelpItem(t, outlinerItems, ViewOutliner, "enter", "Open task detail")
	assertHasHelpItem(t, outlinerItems, ViewOutliner, "shift+enter", "Peek task detail")
	assertHasHelpItem(t, outlinerItems, ViewOutliner, "ctrl+n", "Task composer")
	assertLacksHelpItem(t, outlinerItems, ViewOutliner, "ctrl+o", "Open task reference copy")
	assertLacksHelpItem(t, outlinerItems, ViewOutliner, "w", "Add to today's plan")
	if count := countHelpItem(outlinerItems, "ctrl+n", "Task composer"); count != 1 {
		t.Fatalf("Outliner help has %d ctrl+n Task composer rows, want 1", count)
	}

	allTasksItems := (&AllTasksViewModel{}).GetHelpItems()
	assertHasHelpItem(t, allTasksItems, ViewAllTasks, "shift+enter", "Peek task detail")
	assertHasHelpItem(t, allTasksItems, ViewAllTasks, "w", "Toggle today's plan")
	assertHasHelpItem(t, allTasksItems, ViewAllTasks, "y", "Copy title")

	detail := NewTaskDetailModel(project.Tasks[0], project, app.runtime)
	detailItems := detail.GetHelpItems()
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "j/k", "Scroll down/up")
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "q", "Toggle queue")
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "w", "Toggle today's plan")
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "ctrl+n", "Task composer")
	assertLacksHelpItem(t, detailItems, ViewTaskDetail, "ctrl+o", "Open task reference copy")
	assertHasHelpItem(t, detailItems, ViewTaskDetail, "y", "Copy title")
	assertLacksHelpItem(t, detailItems, ViewTaskDetail, "shift+enter", "Peek task detail")
}

func TestFocusBoardSimpleKeyGrammarHandledKeysAndHelp(t *testing.T) {
	m, _, _ := focusBoardTaskActionFixture(t)
	keys := m.HandledKeys()
	want := []string{
		"j", "down", "k", "up", "g", "G", "ctrl+u", "ctrl+d", "n", "N",
		"enter", "shift+enter", "space", "/", "v", "a", "e", "x", "X", "s", "p",
		"!", "d", "t", "Z", "m", "J", "K", "P", "y", "u", "ctrl+r", "tab", "shift+tab",
	}
	if !slices.Equal(keys, want) {
		t.Fatalf("Planner keymap snapshot = %v, want %v", keys, want)
	}

	items := m.GetHelpItems()
	for _, expected := range []HelpItem{
		{Key: "enter", Description: "Open task detail"},
		{Key: "shift+enter", Description: "Peek at task"},
		{Key: "space", Description: "Toggle selection"},
		{Key: "v", Description: "Open View menu"},
		{Key: "x", Description: "Complete task"},
		{Key: "J", Description: "Reorder down within Today or Next"},
		{Key: "tab", Description: "Next Planner tab"},
		{Key: "shift+tab", Description: "Previous Planner tab"},
	} {
		assertHasHelpItem(t, items, ViewFocusBoard, expected.Key, expected.Description)
	}
	for _, key := range []string{
		"q", "w", "o", "gg", "gp", "fp", "fa", "1", "2", "3", "4", "5", "0", "tn", "tN", ",d",
	} {
		for _, item := range items {
			if item.Key == key {
				t.Errorf("Planner help advertises retired key %q: %+v", key, item)
			}
		}
	}
}

func TestFocusBoardSimpleKeyGrammarRemovedAppAliasesFallThrough(t *testing.T) {
	app, project := newTestAppModelWithProject()
	view := &ctrlORecorderTaskRefView{task: project.Tasks[0], project: project}
	app.views.Register(ViewFocusBoard, view)
	app.viewStack = []PresentationID{ViewDashboard, ViewFocusBoard}
	app.enableTypedNavigation()
	app.focusQueueIDs = map[string]bool{}
	app.dailyPlanIDs = map[string]bool{}

	// The Planner owns its complete shared task grammar in the view dispatcher.
	// App-level legacy aliases must therefore fall through.
	removed := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"queue", syntheticKey('q')},
		{"today", syntheticKey('w')},
		{"reference", syntheticCtrlKey('o')},
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
	if handled, cmd := app.handleGlobalKeyPress(syntheticCtrlKey('n')); !handled || cmd == nil {
		t.Fatal("FocusBoard should route canonical Ctrl+N to Task Composer")
	}
}

func TestGetGlobalBindings_ExcludesRemovedSemicolonShortcuts(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetGlobalBindings()
	for _, item := range items {
		if strings.HasPrefix(item.Key, ";") {
			t.Fatalf("global bindings still advertise removed prefix %q", item.Key)
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
	for _, view := range []PresentationID{ViewOutliner, ViewCrossProjectOutliner} {
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

	assertHelpItem := func(view PresentationID, key, desc string) {
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

	for _, view := range []PresentationID{ViewOutliner, ViewCrossProjectOutliner} {
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

func TestGetForView_OutlinerUsesCtrlNForTaskComposer(t *testing.T) {
	r := NewKeybindingRegistry()

	for _, view := range []PresentationID{ViewOutliner} {
		items := r.GetForView(view)
		foundCtrlN := false
		for _, item := range items {
			if item.Key == "ctrl+n" && item.Description == "Task composer" {
				foundCtrlN = true
			}
			if item.Key == "ctrl+n" && item.Description == "Open project Notes" {
				t.Fatalf("%s still advertises retired Ctrl+N Notes action", view)
			}
			if item.Key == "ctrl+o" && item.Description == "Open outliner view" {
				t.Fatalf("%s should not advertise ctrl+o as Outliner", view)
			}
			if item.Key == "{" && item.Description == "Open project Notes" {
				t.Fatalf("%s should not advertise hidden Notes alias {", view)
			}
		}
		if !foundCtrlN {
			t.Fatalf("%s missing ctrl+n Task composer binding", view)
		}
	}
}

func TestGetForView_ProjectNotesScopeAdvertisesCanonicalComposerCtrlN(t *testing.T) {
	r := NewKeybindingRegistry()

	for _, view := range projectNotesViews() {
		items := r.GetForView(view)
		for _, item := range items {
			if item.Key == "ctrl+n" && item.Description != "Task composer" {
				t.Fatalf("%s advertises ctrl+n as %q", view, item.Description)
			}
		}
	}
}

func TestGetForView_WeeklyReviewAdvertisesGlobalComposerCtrlN(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewWeeklyReview)

	foundComposer := false
	for _, item := range items {
		if item.Key != "ctrl+n" {
			continue
		}
		if item.Description == "Task composer" {
			foundComposer = true
			continue
		}
		if item.Description == "Next unreviewed section" {
			t.Fatalf("Weekly Review should not advertise hidden local ctrl+n binding %q", item.Description)
		}
		t.Fatalf("Weekly Review advertises unexpected ctrl+n binding %q", item.Description)
	}
	if !foundComposer {
		t.Fatal("Weekly Review should advertise global Ctrl+N Task composer binding")
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
		{name: "Planner", keys: (&FocusBoardModel{}).HandledKeys()},
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

	for _, view := range []PresentationID{ViewDashboard, ViewOutliner, ViewPriorityView, ViewInboxProcessor} {
		items := r.GetForView(view)
		foundQuit := false
		foundHelp := false
		for _, item := range items {
			if item.Key == "ctrl+c" && item.Description == "Quit" {
				foundQuit = true
			}
			if item.Key == "?" && item.Description == "Help" {
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
		if item.Key == "ctrl+esc" && item.Description == "Home" {
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
		views    []PresentationID
		target   PresentationID
		expected bool
	}{
		{[]PresentationID{ViewDashboard, ViewOutliner}, ViewDashboard, true},
		{[]PresentationID{ViewDashboard, ViewOutliner}, ViewPriorityView, false},
		{[]PresentationID{}, ViewDashboard, false},
		{[]PresentationID{ViewOutliner}, ViewOutliner, true},
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
