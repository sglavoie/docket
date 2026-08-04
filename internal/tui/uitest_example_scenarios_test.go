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
		"cleanup-undo-redo.json",
		"daily-loop.json",
		"fresh-install-capture.json",
		"fresh-install-default.json",
		"filter-chord-priority.json",
		"filter-chord-states.json",
		"filter-chord-clear.json",
		"context-hub-open.json",
		"context-hub-regular-search-apply.json",
		"project-hub-boundary-filter.json",
		"project-boundary-create-and-park.json",
		"project-boundary-parked-focus.json",
		"project-boundary-delete-reassigns.json",
		"project-boundary-minimum-layout.json",
		"mode-catalog.json",
		"notes-tool.json",
		"notes-scope-picker.json",
		"notes-search.json",
		"notes-location-pickers.json",
		"global-notes-resize-minimum-layout.json",
		"rich-tools.json",
		"outliner-peek-wide-layout.json",
		"outliner-peek-narrow-layout.json",
		"outliner-bulk-action-intersection.json",
		"outliner-range-selection.json",
		"outliner-space-action-catalog.json",
		"search-restoration.json",
		"state-chord.json",
		"theme-toggle.json",
		"in-app-help.json",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(exampleScenariosDir, name)); err != nil {
			t.Errorf("required final-product scenario %q: %v", name, err)
		}
	}

	retired := []string{
		"command-action-help.json",
		"outliner-inspector-narrow-layout.json",
		"outliner-inspector-wide-layout.json",
		"promoted-tools.json",
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
