package tui

import (
	"strings"
	"testing"
)

func TestRenderPrefixPopup_EmptyPrefix(t *testing.T) {
	hints := PrefixHints{
		",": StatusPrefixHints,
	}

	result := RenderPrefixPopup("", hints, 200)
	if result != "" {
		t.Errorf("expected empty string for empty prefix, got %q", result)
	}
}

func TestRenderPrefixPopup_UnknownPrefix(t *testing.T) {
	hints := PrefixHints{
		",": StatusPrefixHints,
	}

	result := RenderPrefixPopup("x", hints, 200)
	if result != "" {
		t.Errorf("expected empty string for unknown prefix, got %q", result)
	}
}

func TestRenderPrefixPopup_EmptyHints(t *testing.T) {
	hints := PrefixHints{
		",": []PrefixHint{},
	}

	result := RenderPrefixPopup(",", hints, 200)
	if result != "" {
		t.Errorf("expected empty string for prefix with empty hints, got %q", result)
	}
}

func TestRenderPrefixPopup_StatusPrefix(t *testing.T) {
	hints := PrefixHints{
		",": StatusPrefixHints,
	}

	result := RenderPrefixPopup(",", hints, 200)

	// Should contain the prefix and arrow
	if !strings.Contains(result, ", →") {
		t.Errorf("expected result to contain prefix arrow, got %q", result)
	}

	// Should contain escape hint
	if !strings.Contains(result, "Esc to cancel") {
		t.Errorf("expected result to contain escape hint, got %q", result)
	}

	// Should contain status hints
	for _, hint := range StatusPrefixHints {
		if !strings.Contains(result, hint.Description) {
			t.Errorf("expected result to contain description %q, got %q", hint.Description, result)
		}
		if !strings.Contains(result, "("+hint.Key+")") {
			t.Errorf("expected result to contain key (%s), got %q", hint.Key, result)
		}
	}
}

func TestRenderPrefixPopup_FoldPrefix(t *testing.T) {
	hints := PrefixHints{
		"z": FoldPrefixHints,
	}

	result := RenderPrefixPopup("z", hints, 200)

	// Should contain fold hints
	for _, hint := range FoldPrefixHints {
		if !strings.Contains(result, hint.Description) {
			t.Errorf("expected result to contain description %q, got %q", hint.Description, result)
		}
	}
}

func TestRenderPrefixPopup_NavigationPrefix(t *testing.T) {
	hints := PrefixHints{
		"g": NavigationPrefixHints,
	}

	result := RenderPrefixPopup("g", hints, 200)

	// Should contain navigation hints
	for _, hint := range NavigationPrefixHints {
		if !strings.Contains(result, hint.Description) {
			t.Errorf("expected result to contain description %q, got %q", hint.Description, result)
		}
	}
}

func TestRenderPrefixPopup_ProjectTreeNavigationPrefix(t *testing.T) {
	hints := PrefixHints{
		"g": ProjectTreeNavigationPrefixHints,
	}

	result := RenderPrefixPopup("g", hints, 200)

	// Should contain extended navigation hints
	expectedDescriptions := []string{"first", "active", "backlog", "inbox", "completed"}
	for _, desc := range expectedDescriptions {
		if !strings.Contains(result, desc) {
			t.Errorf("expected result to contain description %q, got %q", desc, result)
		}
	}
}

func TestPrefixHint_Structure(t *testing.T) {
	hint := PrefixHint{Key: "s", Description: "start"}

	if hint.Key != "s" {
		t.Errorf("expected Key to be 's', got %q", hint.Key)
	}
	if hint.Description != "start" {
		t.Errorf("expected Description to be 'start', got %q", hint.Description)
	}
}

func TestStatusPrefixHints_Contents(t *testing.T) {
	expectedKeys := map[string]string{
		"s": "start",
		"d": "done",
		"b": "block",
		"x": "cancel",
		"o": "reopen",
	}

	for _, hint := range StatusPrefixHints {
		if expected, ok := expectedKeys[hint.Key]; ok {
			if hint.Description != expected {
				t.Errorf("expected hint %q to have description %q, got %q", hint.Key, expected, hint.Description)
			}
		}
	}

	if len(StatusPrefixHints) != len(expectedKeys) {
		t.Errorf("expected %d status hints, got %d", len(expectedKeys), len(StatusPrefixHints))
	}
}

func TestFoldPrefixHints_Contents(t *testing.T) {
	expectedKeys := map[string]string{
		"a": "toggle",
		"o": "open",
		"c": "close",
		"R": "expand-all",
		"M": "collapse-all",
		"v": "focus",
	}

	for _, hint := range FoldPrefixHints {
		if expected, ok := expectedKeys[hint.Key]; ok {
			if hint.Description != expected {
				t.Errorf("expected hint %q to have description %q, got %q", hint.Key, expected, hint.Description)
			}
		}
	}

	if len(FoldPrefixHints) != len(expectedKeys) {
		t.Errorf("expected %d fold hints, got %d", len(expectedKeys), len(FoldPrefixHints))
	}
}

func TestNavigationPrefixHints_Contents(t *testing.T) {
	if len(NavigationPrefixHints) != 1 {
		t.Errorf("expected 1 navigation hint, got %d", len(NavigationPrefixHints))
	}

	if NavigationPrefixHints[0].Key != "g" || NavigationPrefixHints[0].Description != "first" {
		t.Errorf("expected (g, first), got (%s, %s)",
			NavigationPrefixHints[0].Key, NavigationPrefixHints[0].Description)
	}
}

func TestProjectTreeNavigationPrefixHints_HasExtendedKeys(t *testing.T) {
	// ProjectTree should have more hints than basic navigation
	if len(ProjectTreeNavigationPrefixHints) <= len(NavigationPrefixHints) {
		t.Errorf("expected ProjectTreeNavigationPrefixHints to have more entries than NavigationPrefixHints")
	}

	// Should include basic "gg" functionality
	foundFirst := false
	for _, hint := range ProjectTreeNavigationPrefixHints {
		if hint.Key == "g" && hint.Description == "first" {
			foundFirst = true
			break
		}
	}
	if !foundFirst {
		t.Error("expected ProjectTreeNavigationPrefixHints to contain 'g' -> 'first'")
	}
}

func TestRenderPrefixPopup_WrapsOnNarrowWidth(t *testing.T) {
	hints := PrefixHints{
		"g": ProjectTreeNavigationPrefixHints,
	}

	// Use narrow width to force wrapping
	result := RenderPrefixPopup("g", hints, 50)

	// Should still contain all hints
	for _, hint := range ProjectTreeNavigationPrefixHints {
		if !strings.Contains(result, hint.Description) {
			t.Errorf("expected result to contain description %q even with narrow width, got %q", hint.Description, result)
		}
	}

	// Should contain escape hint
	if !strings.Contains(result, "Esc to cancel") {
		t.Errorf("expected result to contain escape hint even with narrow width, got %q", result)
	}

	// Should wrap to multiple lines (contains newlines)
	if !strings.Contains(result, "\n") {
		t.Errorf("expected result to wrap to multiple lines with narrow width, got %q", result)
	}
}
