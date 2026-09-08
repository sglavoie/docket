package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// scenarioRepositoryRoot resolves the repository root from this file's own
// source path, so both the committed corpus and a relative TT_TUI_SCENARIO are
// found from whatever package depth this test ends up at.
func scenarioRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the example scenario runner")
	}
	return tuiRepositoryRootFrom(t, thisFile)
}

// exampleScenariosDir is the committed set of agent-facing example scenarios
// documented in docs/tui-testing/README.md. This test drives each of them end to
// end so the examples cannot silently rot: a keybinding rename, a view-name
// change, or a layout regression that breaks a documented scenario fails here.
//
// It resolves from the repository root rather than a fixed relative path, so
// the corpus is found from whatever package depth this test ends up at. A
// wrong depth would not fail loudly: the glob would return nothing.
func exampleScenariosDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(scenarioRepositoryRoot(t), "docs", "tui-testing", "scenarios")
}

func TestCommittedScenarioInventoryCoversFinalProduct(t *testing.T) {
	required := []string{
		"capture-grammar.json",
		"capture-attachment.json",
		"capture-comment.json",
		"capture-due-time.json",
		"capture-child-adoption.json",
		"capture-dependency-cross-project.json",
		"search-dependency-direct.json",
		"outliner-inline-edit-dependency-undo.json",
		"outliner-dependency-unavailable-fallback.json",
		"outliner-task-reference-picker.json",
		"outliner-task-reference-unavailable-row.json",
		"outliner-task-reference-undo.json",
		"goto-relation-walk.json",
		"goto-relation-return-after-mutation.json",
		"goto-relation-esc-order.json",
		"goto-relation-lens-round-trip.json",
		"goto-relation-disabled-row-layout.json",
		"review-key-layout-matrix.json",
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
		"named-lens-ordinal-apply.json",
		"named-lens-machine-switch.json",
		"named-lens-machine-round-trip.json",
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
		"outliner-move-to-project-bulk-undo.json",
		"outliner-move-to-project-bulk-narrow-layout.json",
		"outliner-indent-outdent-brackets.json",
		"outliner-project-header-actions.json",
		"outliner-project-delete-undo.json",
		"outliner-space-action-catalog.json",
		"outline-editor-guard.json",
		"search-restoration.json",
		"outliner-search-history-recall.json",
		"search-compose-filters.json",
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
		"conflict-browser-open.json",
		"conflict-browser-narrow-layout.json",
		"outliner-menu-bar.json",
		"outliner-menu-bar-persist.json",
		"filter-tag-rename.json",
		"filter-tag-merge-undo.json",
		"filter-tag-rename-lens-report.json",
		"filter-tag-rename-stale-plan.json",
		"filter-context-rename-refusals.json",
	}
	scenariosDir := exampleScenariosDir(t)
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(scenariosDir, name)); err != nil {
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
		if _, err := os.Stat(filepath.Join(scenariosDir, name)); !os.IsNotExist(err) {
			t.Errorf("retired scenario %q is still committed", name)
		}
	}
}

// TestExampleScenariosPass loads every committed example scenario, runs it
// against its own fixture, and asserts it passes with a clean result. Set
// TT_TUI_SCENARIO to one path while authoring; the default discovers every
// committed JSON file.
func TestExampleScenariosPass(t *testing.T) {
	scenariosDir := exampleScenariosDir(t)
	var paths []string
	if selected := os.Getenv("TT_TUI_SCENARIO"); selected != "" {
		if !filepath.IsAbs(selected) {
			selected = filepath.Join(scenarioRepositoryRoot(t), selected)
		}
		paths = []string{filepath.Clean(selected)}
	} else {
		var err error
		paths, err = filepath.Glob(filepath.Join(scenariosDir, "*.json"))
		if err != nil {
			t.Fatalf("glob example scenarios: %v", err)
		}
	}
	if len(paths) == 0 {
		t.Fatalf("no example scenarios found in %s", scenariosDir)
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
