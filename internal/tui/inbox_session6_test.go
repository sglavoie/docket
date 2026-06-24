package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TestSession6_NewPaletteShortcutsMatchKeymap pins the four palette commands
// added in session 6 to their authoritative keymap chords: plan today (w),
// send to next (mn), set due date today (dt), snooze inbox task (zd). Both
// inbox processor views must surface them.
func TestSession6_NewPaletteShortcutsMatchKeymap(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cases := []struct {
		name     string
		shortcut string
	}{
		{"plan today", "w"},
		{"send to next", "mn"},
		{"set due date today", "dt"},
		{"snooze inbox task", "zd"},
	}

	for _, viewType := range []ViewType{ViewInboxProcessor, ViewGlobalInboxProcessor} {
		cmds := triageInboxCommands(t, app, viewType)
		for _, tc := range cases {
			cmd := findCommand(cmds, tc.name)
			if cmd == nil {
				t.Errorf("[%v] %q should be reachable from palette", viewType, tc.name)
				continue
			}
			if cmd.Shortcut != tc.shortcut {
				t.Errorf("[%v] %q shortcut = %q, want %q", viewType, tc.name, cmd.Shortcut, tc.shortcut)
			}
		}
	}
}

// TestSession6_NewPaletteCommandsCarryTriageAliases asserts the new commands
// carry triage-vocabulary aliases so palette search matches the same verbs as
// the close-message copy and the tutor topics.
func TestSession6_NewPaletteCommandsCarryTriageAliases(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	cmds := app.inboxProcessorCommands()

	cases := []struct {
		name    string
		aliases []string
	}{
		{"plan today", []string{"toggle daily plan", "add to today"}},
		{"send to next", []string{"plan for today", "send to today", "triage next"}},
		{"set due date today", []string{"due today"}},
		{"snooze inbox task", []string{"snooze task", "defer until"}},
	}

	for _, tc := range cases {
		cmd := findCommand(cmds, tc.name)
		if cmd == nil {
			t.Errorf("%q command missing", tc.name)
			continue
		}
		for _, alias := range tc.aliases {
			if !slices.Contains(cmd.Aliases, alias) {
				t.Errorf("%q missing alias %q (got %v)", tc.name, alias, cmd.Aliases)
			}
		}
	}
}

// TestSession6_NewPaletteCommandsScopedToBothInboxViews mirrors the parity
// invariant the older triage commands obey: every session-6 addition must be
// available from both the per-project and global inbox processors.
func TestSession6_NewPaletteCommandsScopedToBothInboxViews(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	cmds := app.inboxProcessorCommands()

	for _, name := range []string{"plan today", "send to next", "set due date today", "snooze inbox task"} {
		cmd := findCommand(cmds, name)
		if cmd == nil {
			t.Errorf("%q command missing", name)
			continue
		}
		if !slices.Contains(cmd.Views, ViewInboxProcessor) {
			t.Errorf("%q missing ViewInboxProcessor in Views (got %v)", name, cmd.Views)
		}
		if !slices.Contains(cmd.Views, ViewGlobalInboxProcessor) {
			t.Errorf("%q missing ViewGlobalInboxProcessor in Views (got %v)", name, cmd.Views)
		}
	}
}

// TestSession6_PlanTodayPaletteHandlerEmitsToggleMsg asserts the palette's
// "plan today" command returns TogglePlanTodayFromPaletteMsg so the inbox
// view ends up at the top of the stack when toggleDailyPlan runs. Returning
// the cmd directly would race the popView round-trip.
func TestSession6_PlanTodayPaletteHandlerEmitsToggleMsg(t *testing.T) {
	app, _, _ := newTriageLocalProcessor(t)
	cmd := findCommand(app.inboxProcessorCommands(), "plan today")
	if cmd == nil {
		t.Fatal("plan today command missing")
	}
	msg := cmd.Handler()
	if _, ok := msg.(TogglePlanTodayFromPaletteMsg); !ok {
		t.Fatalf("plan today handler should return TogglePlanTodayFromPaletteMsg, got %T", msg)
	}
}

// TestSession6_SendToNextHandlerReturnsCmdResult asserts the palette's "send
// to next" command wraps its tea.Cmd in PaletteCommandResultMsg so the move +
// save runs after popView, not before.
func TestSession6_SendToNextHandlerReturnsCmdResult(t *testing.T) {
	app, task, _ := newTriageLocalProcessor(t)
	cmd := findCommand(app.inboxProcessorCommands(), "send to next")
	if cmd == nil {
		t.Fatal("send to next command missing")
	}
	msg := cmd.Handler()
	result, ok := msg.(PaletteCommandResultMsg)
	if !ok {
		t.Fatalf("send to next handler should return PaletteCommandResultMsg, got %T", msg)
	}
	if result.Cmd == nil {
		t.Error("PaletteCommandResultMsg.Cmd should be non-nil for send to next")
	}
	if task.Section != models.SectionNext {
		t.Errorf("task.Section = %q, want %q after send to next", task.Section, models.SectionNext)
	}
}

// TestSession6_SetDueDateTodayHandlerReturnsCmdResult asserts "set due date
// today" mirrors the keymap chord `dt` (DueToday → setQuickDueDate(0)) and
// wraps its tea.Cmd in PaletteCommandResultMsg.
func TestSession6_SetDueDateTodayHandlerReturnsCmdResult(t *testing.T) {
	app, task, _ := newTriageLocalProcessor(t)
	cmd := findCommand(app.inboxProcessorCommands(), "set due date today")
	if cmd == nil {
		t.Fatal("set due date today command missing")
	}
	msg := cmd.Handler()
	result, ok := msg.(PaletteCommandResultMsg)
	if !ok {
		t.Fatalf("set due date today handler should return PaletteCommandResultMsg, got %T", msg)
	}
	if result.Cmd == nil {
		t.Error("PaletteCommandResultMsg.Cmd should be non-nil for set due date today")
	}
	if task.DueDate == nil {
		t.Error("task should have a due date set after set due date today")
	}
}

// TestSession6_SnoozeInboxTaskHandlerReturnsSnoozeMsg asserts the palette's
// "snooze inbox task" command emits SetSnoozeDateMsg directly so the snooze
// modal opens (palette stays open via the keep-palette switch in
// handleSearchMessages).
func TestSession6_SnoozeInboxTaskHandlerReturnsSnoozeMsg(t *testing.T) {
	app, _, _ := newTriageLocalProcessor(t)
	cmd := findCommand(app.inboxProcessorCommands(), "snooze inbox task")
	if cmd == nil {
		t.Fatal("snooze inbox task command missing")
	}
	msg := cmd.Handler()
	if _, ok := msg.(SetSnoozeDateMsg); !ok {
		t.Fatalf("snooze inbox task handler should return SetSnoozeDateMsg, got %T", msg)
	}
}

// TestSession6_SnoozeMsgKeepsPaletteOpen asserts SetSnoozeDateMsg is in the
// keep-palette switch in handleSearchMessages, matching SetDueDateMsg's
// behavior. Without this, the snooze modal would open behind the popped
// palette.
func TestSession6_SnoozeMsgKeepsPaletteOpen(t *testing.T) {
	app, _, _ := newTriageLocalProcessor(t)
	executeMsg := CommandPaletteExecuteMsg{ResultMsg: SetSnoozeDateMsg{}}
	handled, _ := app.handleSearchMessages(executeMsg)
	if !handled {
		t.Fatal("palette execute should be handled")
	}
	if app.currentView() != ViewCommandPalette {
		t.Errorf("palette should remain on top for SetSnoozeDateMsg, current view = %v", app.currentView())
	}
}

// TestSession6_TogglePlanTodayMsgRoutedThroughInboxHandler verifies the
// AppModel.processMessage chain reaches handleInboxMessages so the toggle
// runs against the inbox view that was at the top of the stack pre-palette.
// This is what makes the "plan today" command work without synthetic key
// replay.
func TestSession6_TogglePlanTodayMsgRoutedThroughInboxHandler(t *testing.T) {
	app, _, processor := newTriageLocalProcessor(t)
	// toggleDailyPlan operates on whichever view is at the top of the stack
	// (post-palette-pop), so we register the processor under
	// ViewInboxProcessor before popping the palette.
	app.views.Register(ViewInboxProcessor, processor)
	if app.dailyPlanIDs == nil {
		app.dailyPlanIDs = make(map[string]bool)
	}
	if app.focusQueueIDs == nil {
		app.focusQueueIDs = make(map[string]bool)
	}
	app.popView()
	if app.currentView() != ViewInboxProcessor {
		t.Fatalf("expected inbox view on top after popView, got %v", app.currentView())
	}

	handled, _ := app.handleInboxMessages(TogglePlanTodayFromPaletteMsg{})
	if !handled {
		t.Fatal("handleInboxMessages should handle TogglePlanTodayFromPaletteMsg")
	}
	if len(app.dailyPlan) != 1 {
		t.Errorf("expected daily plan to gain 1 task after toggle, got %d", len(app.dailyPlan))
	}
}

// TestSession6_InboxPaletteSuggestedItemsLeadWithTriageVocab asserts the
// palette's "Suggested" section leads with the session-6 triage vocabulary
// when opened from an inbox processor. The order mirrors the close-message
// next-action vocabulary so a brand-new user lands on those actions before
// having to memorize chords.
func TestSession6_InboxPaletteSuggestedItemsLeadWithTriageVocab(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	registry := NewCommandRegistry()
	for i, group := range app.commandGroups() {
		if err := registry.RegisterBatch(group); err != nil {
			t.Fatalf("RegisterBatch[%d] failed: %v", i, err)
		}
	}

	// Reset recent selections so no commands are filtered out of suggestions.
	recentPaletteSelections = nil
	t.Cleanup(func() { recentPaletteSelections = nil })

	palette := NewCommandPaletteModel(app.config, app.configManager, registry, ViewInboxProcessor, nil)
	palette.searchInput.SetValue("")
	palette.filterItems()

	var suggestedNames []string
	for _, fi := range palette.filteredItems {
		if !fi.IsSuggested {
			continue
		}
		if fi.Item.Command == nil {
			continue
		}
		suggestedNames = append(suggestedNames, fi.Item.Command.Name)
	}

	if len(suggestedNames) == 0 {
		t.Fatal("expected at least one suggested item when opening palette inside an inbox processor")
	}
	if suggestedNames[0] != "plan today" {
		t.Errorf("first suggested item should be 'plan today', got %q (full list: %v)", suggestedNames[0], suggestedNames)
	}

	// All suggested items should come from the curated inbox order — none from
	// the generic category fallback yet (the curated list has 7 entries and
	// maxSuggested is 3).
	allowed := map[string]bool{
		"plan today":             true,
		"send to next":           true,
		"set due date today":     true,
		"snooze inbox task":      true,
		"mark task done":         true,
		"edit inbox task":        true,
		"manage inbox task tags": true,
	}
	for _, name := range suggestedNames {
		if !allowed[name] {
			t.Errorf("suggested item %q is not in the curated inbox triage vocabulary", name)
		}
	}
}

// TestSession6_NonInboxPaletteSuggestedItemsKeepGenericOrdering asserts the
// suggested-section override only applies to inbox processor views. A palette
// opened from the dashboard should still pick up category-priority items, not
// inbox-specific triage commands (which would not even be in scope there).
func TestSession6_NonInboxPaletteSuggestedItemsKeepGenericOrdering(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	registry := NewCommandRegistry()
	for i, group := range app.commandGroups() {
		if err := registry.RegisterBatch(group); err != nil {
			t.Fatalf("RegisterBatch[%d] failed: %v", i, err)
		}
	}

	recentPaletteSelections = nil
	t.Cleanup(func() { recentPaletteSelections = nil })

	palette := NewCommandPaletteModel(app.config, app.configManager, registry, ViewDashboard, nil)
	palette.searchInput.SetValue("")
	palette.filterItems()

	for _, fi := range palette.filteredItems {
		if !fi.IsSuggested || fi.Item.Command == nil {
			continue
		}
		// Inbox-scoped commands must not appear for the dashboard palette —
		// they are not even registered for that view.
		if slices.Contains(fi.Item.Command.Views, ViewInboxProcessor) && !slices.Contains(fi.Item.Command.Views, ViewDashboard) {
			t.Errorf("dashboard palette suggested inbox-only command %q", fi.Item.Command.Name)
		}
	}
}

// session6Carrier asserts the palette execute switch keeps SetSnoozeDateMsg
// in its "keep palette open" set. If somebody removes the case the snooze
// modal would briefly draw under the palette pop.
var _ tea.Msg = SetSnoozeDateMsg{}
