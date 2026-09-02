package tui

import (
	"os"
	"path/filepath"
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

func TestUITestScenarioValidateCountContains(t *testing.T) {
	valid := UITestScenario{Actions: []UITestAction{{
		Type:   UITestActionAssert,
		Assert: "count_contains",
		Value:  "workspace:",
		Count:  1,
	}}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid count_contains assertion: %v", err)
	}

	for _, action := range []UITestAction{
		{Type: UITestActionAssert, Assert: "count_contains", Count: 1},
		{Type: UITestActionAssert, Assert: "count_contains", Value: "workspace:"},
	} {
		if err := (UITestScenario{Actions: []UITestAction{action}}).Validate(); err == nil {
			t.Fatalf("invalid count_contains action passed validation: %+v", action)
		}
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
