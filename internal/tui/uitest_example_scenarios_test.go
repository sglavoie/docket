package tui

import (
	"path/filepath"
	"testing"
)

// exampleScenariosDir is the committed set of agent-facing example scenarios
// documented in docs/tui-testing/README.md. This test drives each of them end to
// end so the examples cannot silently rot: a keybinding rename, a view-name
// change, or a layout regression that breaks a documented scenario fails here.
const exampleScenariosDir = "../../docs/tui-testing/scenarios"

// TestExampleScenariosPass loads every committed example scenario, runs it
// against its own fixture, and asserts it passes with a clean result. Each
// scenario is self-contained (it names its fixture and size), so no CLI flags
// are involved — this mirrors `tt tui-test run --scenario <file>`.
func TestExampleScenariosPass(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(exampleScenariosDir, "*.json"))
	if err != nil {
		t.Fatalf("glob example scenarios: %v", err)
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
			if err := sc.Validate(); err != nil {
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
				t.Fatalf("example scenario %q failed after %d action(s): %s", path, result.ActionsRun, reason)
			}
			if result.ActionsRun != len(sc.Actions) {
				t.Fatalf("example scenario %q ran %d/%d actions", path, result.ActionsRun, len(sc.Actions))
			}
		})
	}
}
