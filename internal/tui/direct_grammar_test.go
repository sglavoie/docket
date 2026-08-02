package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestFinalDirectGrammarOwnsEveryDocumentedKeyOnce(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	direct := map[string]OperationID{
		"a": operationCoreCapture, "A": operationTaskNewChild,
		"e": operationCoreEdit, "E": operationCoreEditDescription,
		"ctrl+e": operationCoreExternalEditor,
		"space":  operationCoreActions,
		"j":      operationCoreCursorDown, "down": operationCoreCursorDown,
		"k": operationCoreCursorUp, "up": operationCoreCursorUp,
		"g": operationCoreCursorTop, "home": operationCoreCursorTop,
		"G": operationCoreCursorBottom, "end": operationCoreCursorBottom,
		"ctrl+u": operationCoreCursorPageUp, "pgup": operationCoreCursorPageUp,
		"ctrl+d": operationCoreCursorPageDown, "pgdown": operationCoreCursorPageDown,
		"h": operationCoreCollapseParent, "left": operationCoreCollapseParent,
		"l": operationCoreExpand, "right": operationCoreExpand,
		"S": operationCoreToggleSections,
		"J": operationCoreReorderDown, "K": operationCoreReorderUp,
		"<": operationTaskOutdent, ">": operationTaskIndent,
		"enter": operationCoreDetail,
		"/":     operationCoreVisibleSearch, "c": operationCoreScope,
		"C": operationCoreSelectProject,
		"v": operationCoreMultiSelect, "V": operationCoreSelectAll,
		"N": operationCoreNotes, "T": operationCoreToggleTheme,
		"u": operationCoreUndo, "ctrl+r": operationCoreRedo,
		"esc": operationCoreBack, "?": operationCoreHelp, "ctrl+c": operationCoreQuit,
		"tab": operationCoreNextWorkMode, "shift+tab": operationCorePreviousWorkMode,
		"@": operationTaskSetDueDate, "#": operationTaskSetTags,
		"z": operationTaskSetSnooze, "P": operationTaskTogglePin,
		"y": operationTaskCopyTitle, "Y": operationTaskCopyDetails,
		"ctrl+y": operationTaskCopyTitleDescription,
	}

	owners := make(map[string]OperationID)
	for _, binding := range strategy.Bindings {
		for _, key := range binding.Keys {
			if owner, duplicate := owners[key]; duplicate {
				t.Errorf("direct key %q is owned by both %s and %s", key, owner, binding.OperationID)
			}
			owners[key] = binding.OperationID
		}
	}
	if len(owners) != len(direct) {
		t.Fatalf("direct key count = %d, want %d; got %v", len(owners), len(direct), owners)
	}
	for key, operationID := range direct {
		if got := owners[key]; got != operationID {
			t.Errorf("direct key %q = %s, want %s", key, got, operationID)
		}
	}

	for _, removed := range []string{"b", "d", "n", "t", ",", "m", ":", "R", "p", "ctrl+esc", "ctrl+n"} {
		if owner := owners[removed]; owner != "" {
			t.Errorf("removed key %q still resolves to %s", removed, owner)
		}
	}
}

func TestNotesDirectGrammarMatchesTheWorkspaceManager(t *testing.T) {
	view := &NotesViewModel{}
	want := []string{
		"j", "k", "down", "up", "enter", "n", "a", "v", "s", "r", "m", "x", "u", "ctrl+r",
	}
	if got := view.HandledKeys(); !slices.Equal(got, want) {
		t.Fatalf("Notes direct keys = %v, want %v", got, want)
	}
	for _, excluded := range []string{
		"ctrl+n", "space", "p", "t", "/", "ctrl+o",
	} {
		if slices.Contains(view.HandledKeys(), excluded) {
			t.Errorf("Notes direct grammar exposes excluded key %q", excluded)
		}
	}
}

func TestFinalPriorityChordUsesBangPrefix(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	got := make(map[string]any)
	for _, binding := range strategy.Bindings {
		if binding.OperationID != operationTaskSetPriority ||
			len(binding.Sequence) != 2 ||
			binding.Sequence[0] != "!" {
			continue
		}
		got[binding.Sequence[1]] = binding.PresetValue
	}
	want := map[string]any{"0": 0, "1": 1, "2": 2, "3": 3}
	if len(got) != len(want) {
		t.Fatalf("priority chord terminals = %v, want %v", got, want)
	}
	for key, priority := range want {
		if got[key] != priority {
			t.Errorf("!%s = %v, want %v", key, got[key], priority)
		}
	}
	for _, binding := range strategy.Bindings {
		if binding.OperationID == operationTaskSetPriority &&
			len(binding.Sequence) > 0 &&
			binding.Sequence[0] == "p" {
			t.Errorf("retired priority prefix remains registered: %v", binding.Sequence)
		}
	}
}

func TestFinalStateChordOwnsEveryStateAndPlanningAction(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	type terminal struct {
		operation OperationID
		value     any
		hasValue  bool
	}
	got := make(map[string]terminal)
	gotDisplay := make(map[string]string)
	gotOrder := make([]string, 0, 10)
	for _, binding := range strategy.Bindings {
		if len(binding.Sequence) != 2 || binding.Sequence[0] != "s" {
			continue
		}
		key := binding.Sequence[1]
		gotOrder = append(gotOrder, key)
		gotDisplay[key] = binding.DisplayKey
		got[key] = terminal{
			operation: binding.OperationID,
			value:     binding.PresetValue,
			hasValue:  binding.HasPresetValue,
		}
	}
	want := map[string]terminal{
		"i": {operation: operationTaskSetState, value: models.StateInbox, hasValue: true},
		"b": {operation: operationCoreBacklog},
		"n": {operation: operationCoreNext},
		"t": {operation: operationCoreToday},
		"g": {operation: operationTaskSetState, value: models.StateDoing, hasValue: true},
		"w": {operation: operationTaskSetState, value: models.StateWaiting, hasValue: true},
		"p": {operation: operationTaskSetState, value: models.StatePaused, hasValue: true},
		"m": {operation: operationTaskSetState, value: models.StateSomeday, hasValue: true},
		"d": {operation: operationTaskSetState, value: models.StateDone, hasValue: true},
		"c": {operation: operationTaskSetState, value: models.StateCanceled, hasValue: true},
	}
	if len(got) != len(want) {
		t.Fatalf("state chord terminals = %v, want %v", got, want)
	}
	if wantOrder := []string{"i", "b", "n", "t", "g", "w", "p", "m", "d", "c"}; !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("state chord terminal order = %v, want %v", gotOrder, wantOrder)
	}
	for key, terminal := range want {
		if !reflect.DeepEqual(got[key], terminal) {
			t.Errorf("s%s = %+v, want %+v", key, got[key], terminal)
		}
		if gotDisplay[key] != "s"+key {
			t.Errorf("s%s display key = %q, want %q", key, gotDisplay[key], "s"+key)
		}
	}
	for _, binding := range strategy.Bindings {
		if len(binding.Sequence) > 0 && binding.Sequence[0] == "," {
			t.Errorf("retired comma state prefix remains registered: %v", binding.Sequence)
		}
		if reflect.DeepEqual(binding.Sequence, []string{"s", "r"}) {
			t.Errorf("retired sr state alias remains registered")
		}
	}
}

func TestDirectTaskGrammarRemainsDocumentedWithoutACursorTarget(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	items := registry.HelpItemsForInputStrategy(
		InputStrategySimplified,
		simplifiedOperationContext(),
	)
	keys := make(map[string]bool, len(items))
	for _, item := range items {
		keys[item.Key] = true
	}
	for _, key := range []string{"A", "@", "#", "z", "P", "<", ">", "y", "Y", "Ctrl+Y"} {
		if !keys[key] {
			t.Errorf("target-free help omits documented direct key %q", key)
		}
	}
}

func TestTaskOperationsCannotAppearInBothDirectGrammarAndMenu(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	strategy := SimplifiedInputStrategy()
	if err := validateTaskOperationOnePath(strategy); err != nil {
		t.Fatalf("production grammar: %v", err)
	}

	bound := make(map[OperationID]bool)
	for _, binding := range strategy.Bindings {
		bound[binding.OperationID] = true
	}
	menu := make(map[OperationID]bool)
	for _, id := range taskOperationManifestIDs() {
		menu[id] = true
	}
	for _, entry := range taskOperationDescriptorManifest() {
		if _, ok := registry.Descriptor(entry.ID); !ok {
			t.Errorf("manifested task operation %s is not registered", entry.ID)
		}
		if bound[entry.ID] == menu[entry.ID] {
			t.Errorf(
				"registered task operation %s has %d invocation paths; want exactly one",
				entry.ID,
				invocationPathCount(bound[entry.ID])+invocationPathCount(menu[entry.ID]),
			)
		}
	}

	t.Run("duplicate placement", func(t *testing.T) {
		candidate := cloneInputStrategy(strategy)
		candidate.Bindings = append(candidate.Bindings, OperationKeyBinding{
			OperationID: operationTaskMoveProject,
			Keys:        []string{"M"},
		})
		err := validateTaskOperationOnePath(candidate)
		if err == nil || !strings.Contains(err.Error(), "bound directly and listed") {
			t.Fatalf("error = %v, want duplicate-placement rejection", err)
		}
	})

	t.Run("unreachable direct-only operation", func(t *testing.T) {
		candidate := cloneInputStrategy(strategy)
		candidate.Bindings = slices.DeleteFunc(candidate.Bindings, func(binding OperationKeyBinding) bool {
			return binding.OperationID == operationTaskSetSnooze
		})
		err := validateTaskOperationOnePath(candidate)
		if err == nil || !strings.Contains(err.Error(), "direct-only but has no direct binding") {
			t.Fatalf("error = %v, want unreachable-direct rejection", err)
		}
	})
}

func invocationPathCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestDirectThemeAndQuitOperationsUseTheirDescriptors(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	beforeTheme := app.runtime.Machine().Theme
	t.Cleanup(func() {
		ApplyTheme(NewThemeConfig(app.runtime.Machine()))
	})

	themeResult := executeDirectPath(
		t,
		app,
		app.operationCommandContext(),
		[]string{"T"},
	)
	if themeResult.Err != nil {
		t.Fatalf("toggle theme: %v", themeResult.Err)
	}
	timeout, ok := themeResult.Message.(OperationCommandResultMsg)
	if !ok || timeout.Cmd == nil {
		t.Fatalf("toggle theme message = %T, want a status timeout command", themeResult.Message)
	}
	if app.statusMessage != "Theme: "+app.runtime.Machine().Theme {
		t.Fatalf("theme status = %q, want the active theme", app.statusMessage)
	}
	if handled, _ := app.handleViewMessages(StatusClearMsg{ScheduledAt: app.statusUpdatedAt.Add(time.Nanosecond)}); !handled {
		t.Fatal("theme status clear message was not handled")
	}
	if app.statusMessage != "" {
		t.Fatalf("theme status = %q after timeout, want empty", app.statusMessage)
	}
	afterTheme := app.runtime.Machine().Theme
	if afterTheme == beforeTheme || (afterTheme != "dark" && afterTheme != "light") {
		t.Fatalf("theme changed from %q to %q, want the opposite maintained variant", beforeTheme, afterTheme)
	}
	reloaded, err := config.LoadRuntimeState(workspace.DataDir)
	if err != nil {
		t.Fatalf("reload machine theme: %v", err)
	}
	if reloaded.Machine().Theme != afterTheme {
		t.Fatalf("persisted machine theme = %q, want %q", reloaded.Machine().Theme, afterTheme)
	}
	app.pushView(ViewContextHub)
	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{
		Code: 'c',
		Mod:  tea.ModCtrl,
	})
	if !handled || cmd == nil {
		t.Fatalf("Ctrl+C over a subordinate route = handled %v cmd %v, want descriptor dispatch", handled, cmd)
	}
	if _, ok := cmd().(QuitMsg); !ok {
		t.Fatalf("Ctrl+C command emitted %T, want QuitMsg", cmd())
	}
}
