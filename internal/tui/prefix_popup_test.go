package tui

import (
	"strings"
	"testing"
)

func TestRenderPrefixPopup_EmptyPrefix(t *testing.T) {
	hints := PrefixHints{
		",": StatusPrefixHints,
	}

	result := RenderPrefixPopup("", hints, 200, 40)
	if result != "" {
		t.Errorf("expected empty string for empty prefix, got %q", result)
	}
}

func TestRenderPrefixPopup_UnknownPrefix(t *testing.T) {
	hints := PrefixHints{
		",": StatusPrefixHints,
	}

	result := RenderPrefixPopup("x", hints, 200, 40)
	if result != "" {
		t.Errorf("expected empty string for unknown prefix, got %q", result)
	}
}

func TestRenderPrefixPopup_EmptyHints(t *testing.T) {
	hints := PrefixHints{
		",": []PrefixHint{},
	}

	result := RenderPrefixPopup(",", hints, 200, 40)
	if result != "" {
		t.Errorf("expected empty string for prefix with empty hints, got %q", result)
	}
}

func TestRenderPrefixPopup_StatusPrefix(t *testing.T) {
	hints := PrefixHints{
		",": StatusPrefixHints,
	}

	result := RenderPrefixPopup(",", hints, 200, 40)

	// Should contain the prefix and arrow
	if !strings.Contains(result, "→") {
		t.Errorf("expected result to contain arrow, got %q", result)
	}

	// Should contain escape hint
	if !strings.Contains(result, "Esc to cancel") {
		t.Errorf("expected result to contain escape hint, got %q", result)
	}

	// Should contain status hint descriptions
	for _, hint := range StatusPrefixHints {
		if !strings.Contains(result, hint.Description) {
			t.Errorf("expected result to contain description %q, got %q", hint.Description, result)
		}
	}

	// Should have a border (rounded corners)
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╯") {
		t.Errorf("expected result to have rounded border characters, got %q", result)
	}
}

func TestRenderPrefixPopup_FoldPrefix(t *testing.T) {
	hints := PrefixHints{
		"z": FoldPrefixHints,
	}

	result := RenderPrefixPopup("z", hints, 200, 40)

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

	result := RenderPrefixPopup("g", hints, 200, 40)

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

	result := RenderPrefixPopup("g", hints, 200, 40)

	// Should contain extended navigation hints
	expectedDescriptions := []string{"first", "active", "next", "inbox", "waiting", "someday", "completed"}
	for _, desc := range expectedDescriptions {
		if !strings.Contains(result, desc) {
			t.Errorf("expected result to contain description %q, got %q", desc, result)
		}
	}
}

func TestPrefixHint_Structure(t *testing.T) {
	hint := PrefixHint{Key: "s", Description: "start", Detail: "Set task to in-progress"}

	if hint.Key != "s" {
		t.Errorf("expected Key to be 's', got %q", hint.Key)
	}
	if hint.Description != "start" {
		t.Errorf("expected Description to be 'start', got %q", hint.Description)
	}
	if hint.Detail != "Set task to in-progress" {
		t.Errorf("expected Detail to be 'Set task to in-progress', got %q", hint.Detail)
	}
}

func TestStatusPrefixHints_Contents(t *testing.T) {
	expectedKeys := map[string]string{
		"s": "start",
		"d": "done",
		"b": "block",
		"x": "cancel",
		"o": "reopen",
		"r": "review",
		"S": "standby",
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
		"p": "preview",
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

func TestRenderPrefixPopup_CompactModeOnSmallHeight(t *testing.T) {
	hints := PrefixHints{
		"g": ProjectTreeNavigationPrefixHints,
	}

	// Use small height to force compact mode (18 hints won't fit one-per-line)
	result := RenderPrefixPopup("g", hints, 80, 10)

	// Should still contain all hints
	for _, hint := range ProjectTreeNavigationPrefixHints {
		if !strings.Contains(result, hint.Description) {
			t.Errorf("expected result to contain description %q even in compact mode, got %q", hint.Description, result)
		}
	}

	// Should contain escape hint
	if !strings.Contains(result, "Esc to cancel") {
		t.Errorf("expected result to contain escape hint, got %q", result)
	}
}

func TestRenderPrefixPopup_CategoryTitle(t *testing.T) {
	hints := PrefixHints{
		",": StatusPrefixHints,
	}

	result := RenderPrefixPopup(",", hints, 200, 40)

	// Should contain "status" category name
	if !strings.Contains(result, "tatus") {
		t.Errorf("expected result to contain category name 'status', got %q", result)
	}
}

func TestOverlayPopup_EmptyPopup(t *testing.T) {
	base := "line1\nline2\nline3"
	result := OverlayPopup(base, "", 20, 3)
	if result != base {
		t.Errorf("expected base unchanged for empty popup, got %q", result)
	}
}

func TestOverlayPopup_PlacesAtBottom(t *testing.T) {
	base := strings.Repeat("x", 20) + "\n" +
		strings.Repeat("x", 20) + "\n" +
		strings.Repeat("x", 20) + "\n" +
		strings.Repeat("x", 20) + "\n" +
		strings.Repeat("x", 20)
	popup := "POPUP"

	result := OverlayPopup(base, popup, 20, 5)
	lines := strings.Split(result, "\n")

	// Popup should be near the bottom (line 3 of 5, since bottom-1)
	if !strings.Contains(lines[3], "POPUP") {
		t.Errorf("expected popup in line 3, lines: %v", lines)
	}
}

func TestStageKeyForIndex(t *testing.T) {
	tests := []struct {
		index int
		want  string
	}{
		{0, "1"}, {8, "9"},
		{9, "a"}, {10, "b"}, {34, "z"},
		{-1, ""}, {35, ""},
	}
	for _, tt := range tests {
		got := stageKeyForIndex(tt.index)
		if got != tt.want {
			t.Errorf("stageKeyForIndex(%d) = %q, want %q", tt.index, got, tt.want)
		}
	}
}

func TestStageIndexForKey(t *testing.T) {
	tests := []struct {
		key  string
		want int
	}{
		{"1", 0}, {"9", 8},
		{"a", 9}, {"b", 10}, {"z", 34},
		{"0", -1}, {"A", -1}, {"", -1}, {"ab", -1},
	}
	for _, tt := range tests {
		got := stageIndexForKey(tt.key)
		if got != tt.want {
			t.Errorf("stageIndexForKey(%q) = %d, want %d", tt.key, got, tt.want)
		}
	}
}

func TestStageForPrefixKey_Extended(t *testing.T) {
	stages := make([]string, 12)
	for i := range stages {
		stages[i] = strings.Repeat("s", i+1) // "s", "ss", "sss", ...
	}

	// Key "a" should map to stages[9] (10th stage)
	got := StageForPrefixKey("a", stages)
	if got == nil || *got != stages[9] {
		t.Errorf("StageForPrefixKey(\"a\") = %v, want %q", got, stages[9])
	}

	// Key "c" should map to stages[11] (12th stage)
	got = StageForPrefixKey("c", stages)
	if got == nil || *got != stages[11] {
		t.Errorf("StageForPrefixKey(\"c\") = %v, want %q", got, stages[11])
	}

	// Key "d" is out of range (only 12 stages)
	got = StageForPrefixKey("d", stages)
	if got != nil {
		t.Errorf("StageForPrefixKey(\"d\") should be nil for 12 stages, got %q", *got)
	}
}

func TestBuildStagePrefixHints_Extended(t *testing.T) {
	stages := make([]string, 11)
	for i := range stages {
		stages[i] = strings.Repeat("s", i+1)
	}
	hints := BuildStagePrefixHints(stages, "")

	// 11 stage hints + 1 clear hint
	if len(hints) != 12 {
		t.Fatalf("expected 12 hints, got %d", len(hints))
	}
	if hints[0].Key != "1" {
		t.Errorf("hint[0].Key = %q, want \"1\"", hints[0].Key)
	}
	if hints[8].Key != "9" {
		t.Errorf("hint[8].Key = %q, want \"9\"", hints[8].Key)
	}
	if hints[9].Key != "a" {
		t.Errorf("hint[9].Key = %q, want \"a\"", hints[9].Key)
	}
	if hints[10].Key != "b" {
		t.Errorf("hint[10].Key = %q, want \"b\"", hints[10].Key)
	}
	if hints[11].Key != "0" {
		t.Errorf("last hint Key = %q, want \"0\"", hints[11].Key)
	}
}
