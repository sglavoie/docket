package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Parsing -----------------------------------------------------------------

func TestUITestScenarioParseAndValidate(t *testing.T) {
	raw := `{
		"name": "open dashboard",
		"fixture": "rich",
		"size": "120x40",
		"actions": [
			{"type": "assert", "assert": "view", "value": "Dashboard", "label": "starts on dashboard"},
			{"type": "resize", "width": 100, "height": 30},
			{"type": "key", "key": "ctrl+i"},
			{"type": "wait"}
		]
	}`

	sc, err := ParseScenario([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScenario error: %v", err)
	}
	if sc.Name != "open dashboard" || sc.Fixture != "rich" || sc.Size != "120x40" {
		t.Fatalf("parsed header = %q/%q/%q", sc.Name, sc.Fixture, sc.Size)
	}
	if len(sc.Actions) != 4 {
		t.Fatalf("parsed %d actions, want 4", len(sc.Actions))
	}
	if sc.Actions[0].Label != "starts on dashboard" {
		t.Fatalf("action label = %q, want %q", sc.Actions[0].Label, "starts on dashboard")
	}
	if err := sc.Validate(); err != nil {
		t.Fatalf("Validate() error on well-formed scenario: %v", err)
	}
}

func TestUITestScenarioParseRejectsUnknownField(t *testing.T) {
	// A typo'd field name must fail rather than silently drop.
	raw := `{"actions": [{"type": "wait", "kye": "x"}]}`
	if _, err := ParseScenario([]byte(raw)); err == nil {
		t.Fatal("ParseScenario should reject unknown fields")
	}
}

func TestUITestScenarioParseRejectsTrailingContent(t *testing.T) {
	raw := `{"actions": [{"type": "wait"}]} {"extra": true}`
	if _, err := ParseScenario([]byte(raw)); err == nil {
		t.Fatal("ParseScenario should reject trailing content after the first object")
	}
}

func TestUITestScenarioLoadFile(t *testing.T) {
	raw := `{"name": "from file", "actions": [{"type": "wait"}]}`
	path := filepath.Join(t.TempDir(), "scenario.json")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write scenario: %v", err)
	}
	sc, err := LoadScenario(path)
	if err != nil {
		t.Fatalf("LoadScenario error: %v", err)
	}
	if sc.Name != "from file" || len(sc.Actions) != 1 {
		t.Fatalf("LoadScenario got %q/%d actions", sc.Name, len(sc.Actions))
	}
}

// --- Strict validation -------------------------------------------------------

func TestUITestScenarioValidateEmpty(t *testing.T) {
	if err := (UITestScenario{}).Validate(); err == nil {
		t.Fatal("empty scenario should fail validation")
	}
}

func TestUITestScenarioValidateUnknownActionType(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: "teleport"}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("unknown action type should fail validation")
	}
}

func TestUITestScenarioValidateMalformedSize(t *testing.T) {
	// Malformed top-level size.
	sc := UITestScenario{Size: "wide", Actions: []UITestAction{{Type: UITestActionWait}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("malformed top-level size should fail validation")
	}

	// Non-positive resize dimensions.
	sc = UITestScenario{Actions: []UITestAction{{Type: UITestActionResize, Width: 0, Height: 30}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("resize with zero width should fail validation")
	}
}

func TestUITestScenarioValidateInvalidKey(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: UITestActionKey, Key: "ctrl+"}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("invalid key should fail validation before execution")
	}
}

func TestUITestScenarioValidateUnknownAssertKind(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: UITestActionAssert, Assert: "bogus"}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("unknown assertion kind should fail validation")
	}
	sc = UITestScenario{Actions: []UITestAction{{Type: UITestActionAssert}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("missing assertion kind should fail validation")
	}
}

func TestUITestScenarioValidateEmptyText(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: UITestActionText, Text: ""}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("empty text action should fail validation")
	}
}

func TestUITestScenarioValidateRejectsEmptyCommand(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: UITestActionCommand, Command: "  "}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("command action with blank command should fail validation")
	}
}

// --- Size resolution ---------------------------------------------------------

func TestUITestScenarioResolveSize(t *testing.T) {
	// Explicit size wins over defaults.
	w, h, err := (UITestScenario{Size: "80x24"}).ResolveSize(100, 30)
	if err != nil || w != 80 || h != 24 {
		t.Fatalf("ResolveSize(explicit) = %dx%d err=%v, want 80x24", w, h, err)
	}
	// Empty size falls back to defaults.
	w, h, err = (UITestScenario{}).ResolveSize(100, 30)
	if err != nil || w != 100 || h != 30 {
		t.Fatalf("ResolveSize(default) = %dx%d err=%v, want 100x30", w, h, err)
	}
	// Malformed size errors.
	if _, _, err := (UITestScenario{Size: "100"}).ResolveSize(100, 30); err == nil {
		t.Fatal("ResolveSize(malformed) should error")
	}
}

// --- Execution: golden-ish result --------------------------------------------

func TestUITestScenarioRunFixtureGolden(t *testing.T) {
	raw := `{
		"name": "rich dashboard renders stable content",
		"fixture": "rich",
		"size": "120x40",
		"actions": [
			{"type": "assert", "assert": "view", "value": "Dashboard"},
			{"type": "assert", "assert": "contains", "value": "Rich Project"},
			{"type": "assert", "assert": "contains", "value": "Launch platform"},
			{"type": "assert", "assert": "no_layout_issues"}
		]
	}`
	sc, err := ParseScenario([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScenario error: %v", err)
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{IncludeObservations: true})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("scenario should pass, got error: %+v", result.Error)
	}
	if result.Scenario != "rich dashboard renders stable content" {
		t.Fatalf("result scenario name = %q", result.Scenario)
	}
	if result.ActionsRun != 4 {
		t.Fatalf("actions_run = %d, want 4", result.ActionsRun)
	}
	if len(result.Observations) != 4 {
		t.Fatalf("observations = %d, want 4", len(result.Observations))
	}
	if result.FinalObservation == nil {
		t.Fatal("final_observation should be populated")
	}
	if result.FinalObservation.Width != 120 || result.FinalObservation.Height != 40 {
		t.Fatalf("final observation size = %dx%d, want 120x40",
			result.FinalObservation.Width, result.FinalObservation.Height)
	}

	// Golden-ish: the normalized screen carries stable fixture content and no
	// ANSI escapes.
	screen := result.FinalObservation.PlainScreen
	for _, want := range []string{"Rich Project", "Launch platform"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("final screen missing stable token %q:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, "\x1b") {
		t.Fatal("final screen still contains ANSI escape sequences")
	}
}

func TestUITestScenarioRoadmapLayoutNoOverflow(t *testing.T) {
	raw := `{
		"name": "rich roadmap layout",
		"fixture": "rich",
		"size": "100x35",
		"actions": [
			{"type": "command", "command": "roadmap"},
			{"type": "assert", "assert": "view", "value": "Roadmap"},
			{"type": "assert", "assert": "no_layout_issues"}
		]
	}`
	sc, err := ParseScenario([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScenario error: %v", err)
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("roadmap layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioRunFixtureDefaults(t *testing.T) {
	// No fixture or size in the scenario: RunScenarioFixture falls back to the
	// package defaults (rich fixture at 100x30).
	sc := UITestScenario{
		Name:    "defaults",
		Actions: []UITestAction{{Type: UITestActionAssert, Assert: "view", Value: "Dashboard"}},
	}
	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("scenario should pass, got %+v", result.Error)
	}
	if result.FinalObservation == nil ||
		result.FinalObservation.Width != defaultScenarioWidth ||
		result.FinalObservation.Height != defaultScenarioHeight {
		t.Fatalf("default size not applied: %+v", result.FinalObservation)
	}
}

func TestUITestScenarioRunResultOnFailure(t *testing.T) {
	// First action passes, second fails: the result must name the failing step.
	sc := UITestScenario{
		Name:    "failing assert",
		Fixture: "rich",
		Actions: []UITestAction{
			{Type: UITestActionAssert, Assert: "view", Value: "Dashboard"},
			{Type: UITestActionAssert, Assert: "contains", Value: "zzz-absent-token", Label: "expects missing token"},
		},
	}
	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if result.OK {
		t.Fatal("scenario should have failed")
	}
	if result.ActionsRun != 1 {
		t.Fatalf("actions_run = %d, want 1 (one action passed before failure)", result.ActionsRun)
	}
	if result.Error == nil {
		t.Fatal("result.Error should be populated on failure")
	}
	if result.Error.Index != 1 {
		t.Fatalf("error index = %d, want 1", result.Error.Index)
	}
	if result.Error.Label != "expects missing token" {
		t.Fatalf("error label = %q, want %q", result.Error.Label, "expects missing token")
	}
	if result.FinalObservation == nil {
		t.Fatal("final_observation should carry the failure screen")
	}
}

func TestUITestScenarioRunFixtureRejectsInvalidScenario(t *testing.T) {
	// Validation happens before any driver is built, so no workspace leaks.
	_, ws, err := RunScenarioFixture(UITestScenario{}, RunScenarioOptions{})
	if err == nil {
		t.Fatal("empty scenario should fail before execution")
	}
	if ws != nil {
		_ = ws.Cleanup()
		t.Fatal("no workspace should be returned when validation fails")
	}
}

// --- Result JSON contract ----------------------------------------------------

func TestUITestScenarioResultJSONShape(t *testing.T) {
	sc := UITestScenario{
		Name:    "shape",
		Actions: []UITestAction{{Type: UITestActionAssert, Assert: "view", Value: "Dashboard"}},
	}
	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{IncludeObservations: true})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	for _, key := range []string{"ok", "scenario", "actions_run", "final_observation", "observations"} {
		if _, present := decoded[key]; !present {
			t.Fatalf("result JSON missing %q key: %s", key, data)
		}
	}
	// error is omitted on success.
	if _, present := decoded["error"]; present {
		t.Fatalf("result JSON should omit error on success: %s", data)
	}
}
