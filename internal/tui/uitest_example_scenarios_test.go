package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// scenarioRepositoryRoot resolves the repository root from this file's own
// source path, so both the committed corpus and a relative DOCKET_TUI_SCENARIO are
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
		"check-in-integrated-lifecycle.json",
		"check-in-flags-and-stale.json",
		"capture-grammar.json",
		"capture-attachment.json",
		"capture-mail-link.json",
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
		"form-viewport-minimum-height.json",
		"form-extras-comments-narrow.json",
		"cleanup-undo-redo.json",
		"undo-focus-committed-target.json",
		"undo-focus-restart-uncorrelated.json",
		"undo-focus-single-delete.json",
		"daily-loop.json",
		"inbox-triage-advance-across-project-groups.json",
		"inbox-triage-advance-promotes-children.json",
		"inbox-triage-undo-returns-to-task.json",
		"relocation-recovery-subtree-move.json",
		"fresh-install-capture.json",
		"fresh-install-default.json",
		"filter-chord-priority.json",
		"filter-chord-states.json",
		"filter-chord-hierarchy.json",
		"filter-chord-clear.json",
		"sort-chord-axis.json",
		"sort-orders-pinned.json",
		"sort-fixed-order-modes.json",
		"sort-project-group-order.json",
		"named-lens-ordinal-apply.json",
		"named-lens-machine-switch.json",
		"named-lens-machine-round-trip.json",
		"defer-wake-marker.json",
		"date-picker-preset-keys.json",
		"date-picker-empty-field-opening.json",
		"date-picker-section-switch.json",
		"date-picker-planning-persist.json",
		"date-picker-frozen-cross-project-selection.json",
		"date-picker-hidden-selected-members.json",
		"date-picker-narrow-resize-key-confinement.json",
		"deadline-step-keys.json",
		"deadline-step-bulk-hidden-narrow.json",
		"combined-promote-deadline-step-move.json",
		"waiting-check-back-transaction-boundary.json",
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
		"outliner-peek-whole-title.json",
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
		"outliner-escape-guidance-ladder.json",
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
		"menu-over-dimmed-host.json",
		"outliner-menu-bar.json",
		"outliner-menu-bar-persist.json",
		"filter-tag-rename.json",
		"filter-tag-merge-undo.json",
		"filter-tag-rename-lens-report.json",
		"filter-tag-rename-stale-plan.json",
		"filter-context-rename-refusals.json",
		"filter-tag-rename-reopens-facet.json",
		"lens-rename-resume-undo-order.json",
		"selection-fold-copy-hidden-members.json",
		"stale-rename-versus-resume-activity.json",
		// The UX refinement baselines. They are required because their whole value
		// is being the recorded "before": a later session that improves a row, a
		// Peek, a narrow layout, or a selection has to change one of these on
		// purpose rather than delete it.
		"ux-acceptance-row-baseline.json",
		"ux-acceptance-peek-baseline.json",
		"ux-acceptance-narrow-baseline.json",
		"ux-acceptance-selection-baseline.json",
		"ux-acceptance-daily-loop.json",
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

// scenarioShard reads DOCKET_TUI_SCENARIO_SHARD, "i/n" with 0 <= i < n, which
// scripts/test_sharded.sh sets so each of its n processes replays a disjoint
// slice of the corpus. Unset means the whole corpus, as one shard of one.
func scenarioShard(t *testing.T) (index, count int) {
	t.Helper()
	raw := os.Getenv("DOCKET_TUI_SCENARIO_SHARD")
	if raw == "" {
		return 0, 1
	}
	if _, err := fmt.Sscanf(raw, "%d/%d", &index, &count); err != nil || count < 1 || index < 0 || index >= count {
		t.Fatalf("DOCKET_TUI_SCENARIO_SHARD=%q, want i/n with 0 <= i < n", raw)
	}
	return index, count
}

// TestExampleScenariosPass loads every committed example scenario, runs it
// against its own fixture, and asserts it passes with a clean result. Set
// DOCKET_TUI_SCENARIO to one path while authoring; the default discovers every
// committed JSON file, of which DOCKET_TUI_SCENARIO_SHARD keeps one slice.
func TestExampleScenariosPass(t *testing.T) {
	scenariosDir := exampleScenariosDir(t)
	var paths []string
	if selected := os.Getenv("DOCKET_TUI_SCENARIO"); selected != "" {
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
		if index, count := scenarioShard(t); count > 1 {
			var slice []string
			for i, path := range paths {
				if i%count == index {
					slice = append(slice, path)
				}
			}
			if len(slice) == 0 {
				t.Skipf("scenario shard %d/%d holds no scenarios", index, count)
			}
			paths = slice
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
