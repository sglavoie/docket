package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestPendingChordStoresHintsExecutesAndClears(t *testing.T) {
	newChordApp := func(t *testing.T) *AppModel {
		t.Helper()
		app := newUITestApp()
		app.operationRegistry = NewOperationRegistry()
		app.operationExecutor = NewOperationExecutor(app.operationRegistry)
		if err := app.registerCoreInputOperations(); err != nil {
			t.Fatalf("registerCoreInputOperations: %v", err)
		}
		if err := app.operationRegistry.RegisterInputStrategy(InputStrategy{
			ID: InputStrategySimplified,
			Bindings: []OperationKeyBinding{
				{OperationID: operationCoreCapture, Keys: []string{"a"}},
				{OperationID: operationCoreHelp, Sequence: []string{",", "h"}, HintLabel: "help"},
			},
			ChordGroups: []OperationChordGroup{{Sequence: []string{","}, HintLabel: "state"}},
		}); err != nil {
			t.Fatalf("RegisterInputStrategy: %v", err)
		}
		app.wireOutlinerOperationExecutor(app.crossOutliner)
		return app
	}
	comma := tea.KeyPressMsg{Code: ',', Text: ","}

	t.Run("prefix waits and shows its which-key row", func(t *testing.T) {
		app := newChordApp(t)
		handled, cmd := app.handleSimplifiedInputKey(comma)
		if !handled || cmd != nil {
			t.Fatalf("chord prefix = handled %v, cmd %v; want consumed with no command", handled, cmd)
		}
		if !slices.Equal(app.pendingChord, []string{","}) {
			t.Fatalf("pending chord = %v, want [,]", app.pendingChord)
		}
		row := pendingChordHintRowForTest(app)
		if !strings.Contains(row, "h help") {
			t.Errorf("which-key row %q omits the chord's hint", row)
		}
		if !strings.Contains(ansi.Strip(app.renderStatusBar()), "h help") {
			t.Error("the status bar footer does not show the pending chord")
		}
	})

	t.Run("terminal key executes and clears", func(t *testing.T) {
		app := newChordApp(t)
		app.handleSimplifiedInputKey(comma)
		handled, cmd := app.handleSimplifiedInputKey(tea.KeyPressMsg{Code: 'h', Text: "h"})
		if !handled || cmd == nil {
			t.Fatalf("chord completion = handled %v, cmd %v; want the operation's command", handled, cmd)
		}
		if len(app.pendingChord) != 0 {
			t.Errorf("pending chord = %v, want cleared", app.pendingChord)
		}
	})

	t.Run("esc unknown key and reset clear the prefix", func(t *testing.T) {
		for name, abandon := range map[string]func(app *AppModel){
			"esc":     func(app *AppModel) { app.handleSimplifiedInputKey(tea.KeyPressMsg{Code: tea.KeyEscape}) },
			"unknown": func(app *AppModel) { app.handleSimplifiedInputKey(tea.KeyPressMsg{Code: 'z', Text: "z"}) },
			"reset":   func(app *AppModel) { jumpToOutlinerRootForTest(app) },
			"inline control": func(app *AppModel) {
				app.crossOutliner.ActivateSearch()
				app.handleSimplifiedInputKey(tea.KeyPressMsg{Code: 'h', Text: "h"})
			},
		} {
			app := newChordApp(t)
			app.handleSimplifiedInputKey(comma)
			abandon(app)
			if len(app.pendingChord) != 0 {
				t.Errorf("%s left pending chord %v, want cleared", name, app.pendingChord)
			}
		}
	})
}

func TestSimplifiedDispatchConsumesLegacyKeysAndDefersInlineSearch(t *testing.T) {
	app := newUITestApp()
	app.operationRegistry = NewOperationRegistry()
	app.operationExecutor = NewOperationExecutor(app.operationRegistry)
	if err := app.registerCoreInputOperations(); err != nil {
		t.Fatalf("registerCoreInputOperations: %v", err)
	}
	if err := app.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}
	if err := app.registerFilterOperations(); err != nil {
		t.Fatalf("registerFilterOperations: %v", err)
	}
	if err := app.registerSortOperations(); err != nil {
		t.Fatalf("registerSortOperations: %v", err)
	}
	if err := app.registerProjectOperations(); err != nil {
		t.Fatalf("registerProjectOperations: %v", err)
	}
	if err := app.registerProjectActionSheetOperation(); err != nil {
		t.Fatalf("registerProjectActionSheetOperation: %v", err)
	}
	if err := app.registerChecklistRowOperations(); err != nil {
		t.Fatalf("registerChecklistRowOperations: %v", err)
	}
	if err := app.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("RegisterInputStrategy: %v", err)
	}
	app.wireOutlinerOperationExecutor(app.crossOutliner)

	// s and t are absent: the three-toggle fold family took both back, s for
	// section headers and t for task subtrees.
	for _, retired := range []string{
		"q", "i", "|", "ctrl+x",
		":", "R", "ctrl+esc", "m", "b", ",",
	} {
		key, err := parseUITestKey(retired)
		if err != nil {
			t.Fatalf("parse retired key %q: %v", retired, err)
		}
		handled, cmd := app.handleGlobalKeyPress(key)
		if !handled || cmd != nil {
			t.Errorf("retired key %q = handled %v, cmd %v; want consumed no-op", retired, handled, cmd)
		}
	}

	app.crossOutliner.ActivateSearch()
	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if handled || cmd != nil {
		t.Fatalf("search input a = handled %v, cmd %v; want delegated to inline input", handled, cmd)
	}
}

func TestSimplifiedHelpContainsNoRetiredShortcutAliases(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	items := registry.HelpItemsForInputStrategy(InputStrategySimplified, simplifiedOperationContext())
	keys := make(map[string]bool, len(items))
	for _, item := range items {
		keys[item.Key] = true
	}
	for _, retired := range []string{
		"q", "i", "x", "|", "Ctrl+X",
		":", "R", "Ctrl+Esc", "m",
	} {
		if keys[retired] {
			t.Errorf("simplified help retains retired shortcut alias %q", retired)
		}
	}
}
