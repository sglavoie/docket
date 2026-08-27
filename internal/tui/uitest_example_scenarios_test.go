package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// exampleScenariosDir is the committed set of agent-facing example scenarios
// documented in docs/tui-testing/README.md. This test drives each of them end to
// end so the examples cannot silently rot: a keybinding rename, a view-name
// change, or a layout regression that breaks a documented scenario fails here.
const exampleScenariosDir = "../../docs/tui-testing/scenarios"

func TestCommittedScenarioInventoryCoversFinalProduct(t *testing.T) {
	required := []string{
		"capture-grammar.json",
		"capture-attachment.json",
		"capture-comment.json",
		"capture-due-time.json",
		"capture-child-adoption.json",
		"capture-pin.json",
		"capture-project-header-destination.json",
		"form-extras-attachments-narrow.json",
		"form-extras-attachments-minimum-layout.json",
		"form-extras-comments-narrow.json",
		"cleanup-undo-redo.json",
		"daily-loop.json",
		"inbox-triage-advance-across-project-groups.json",
		"inbox-triage-advance-promotes-children.json",
		"inbox-triage-undo-does-not-advance.json",
		"fresh-install-capture.json",
		"fresh-install-default.json",
		"filter-chord-priority.json",
		"filter-chord-states.json",
		"filter-chord-hierarchy.json",
		"filter-chord-clear.json",
		"sort-chord-axis.json",
		"sort-manual-order-preserved.json",
		"sort-fixed-order-modes.json",
		"sort-project-group-order.json",
		"defer-wake-marker.json",
		"project-header-pressure.json",
		"project-header-pressure-narrow.json",
		"mode-catalog.json",
		"notes-tool.json",
		"notes-scope-picker.json",
		"notes-project-header-scope.json",
		"notes-search.json",
		"notes-location-pickers.json",
		"global-notes-resize-minimum-layout.json",
		"rich-tools.json",
		"outliner-peek-wide-layout.json",
		"outliner-peek-narrow-layout.json",
		"outliner-peek-title-continuation.json",
		"outliner-bulk-action-intersection.json",
		"outliner-checklist-rows-fold.json",
		"outliner-checklist-selection-suppression.json",
		"outliner-inline-edit-child-adoption.json",
		"outliner-range-selection.json",
		"outliner-set-parent-picker.json",
		"outliner-adopt-children-picker.json",
		"outliner-adopt-children-additive.json",
		"outliner-adopt-children-cross-project.json",
		"outliner-adopt-children-undo.json",
		"outliner-move-to-project-subtree.json",
		"outliner-move-to-project-undo.json",
		"outliner-move-to-project-parked.json",
		"outliner-indent-outdent-brackets.json",
		"outliner-project-header-actions.json",
		"outliner-project-delete-undo.json",
		"outliner-space-action-catalog.json",
		"outline-editor-guard.json",
		"search-restoration.json",
		"outliner-search-sigils.json",
		"outliner-search-sigil-guidance.json",
		"state-chord.json",
		"task-context-reserved-segment-narrow.json",
		"task-context-row-narrow-layout.json",
		"task-context-row-wide-layout.json",
		"task-context-tag-facet.json",
		"theme-toggle.json",
		"in-app-help.json",
		"project-hub-task-preview.json",
		"project-hub-metadata-search.json",
		"outliner-rich-search-autocomplete.json",
		"notes-responsive-preview.json",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(exampleScenariosDir, name)); err != nil {
			t.Errorf("required final-product scenario %q: %v", name, err)
		}
	}

	retired := []string{
		"command-action-help.json",
		"context-hub-open.json",
		"context-hub-regular-search-apply.json",
		"context-hub-editor-scope-filter.json",
		"context-hub-minimum-layout.json",
		"outliner-inspector-narrow-layout.json",
		"outliner-inspector-wide-layout.json",
		"promoted-tools.json",
		"snooze-custom-entry-bulk.json",
		"snooze-picker-direct.json",
		"snooze-picker-minimum-layout.json",
		"snooze-wake-marker.json",
		"task-composer-minimum-layout.json",
		"task-detail-narrow-layout.json",
	}
	for _, name := range retired {
		if _, err := os.Stat(filepath.Join(exampleScenariosDir, name)); !os.IsNotExist(err) {
			t.Errorf("retired scenario %q is still committed", name)
		}
	}
}

// TestExampleScenariosPass loads every committed example scenario, runs it
// against its own fixture, and asserts it passes with a clean result. Set
// TT_TUI_SCENARIO to one path while authoring; the default discovers every
// committed JSON file.
func TestExampleScenariosPass(t *testing.T) {
	var paths []string
	if selected := os.Getenv("TT_TUI_SCENARIO"); selected != "" {
		if !filepath.IsAbs(selected) {
			selected = filepath.Join("../..", selected)
		}
		paths = []string{filepath.Clean(selected)}
	} else {
		var err error
		paths, err = filepath.Glob(filepath.Join(exampleScenariosDir, "*.json"))
		if err != nil {
			t.Fatalf("glob example scenarios: %v", err)
		}
	}
	if len(paths) == 0 {
		t.Fatalf("no example scenarios found in %s", exampleScenariosDir)
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			sc, err := LoadScenario(path)
			if err != nil {
				t.Fatalf("LoadScenario(%q): %v", path, err)
			}
			if err = sc.Validate(); err != nil {
				t.Fatalf("scenario %q failed validation: %v", path, err)
			}

			result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
			if ws != nil {
				defer func() { _ = ws.Cleanup() }()
			}
			if err != nil {
				t.Fatalf("RunScenarioFixture(%q): %v", path, err)
			}
			if !result.OK {
				reason := "unknown"
				if result.Error != nil {
					reason = result.Error.Error()
				}
				screen := ""
				if result.FinalObservation != nil {
					screen = "\n\nFinal screen:\n" + result.FinalObservation.PlainScreen
				}
				t.Fatalf("example scenario %q failed after %d action(s): %s%s", path, result.ActionsRun, reason, screen)
			}
			if result.ActionsRun != len(sc.Actions) {
				t.Fatalf("example scenario %q ran %d/%d actions", path, result.ActionsRun, len(sc.Actions))
			}
		})
	}
}
