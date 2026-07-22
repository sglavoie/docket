package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/keymap"
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
		"f": FoldPrefixHints,
	}

	result := RenderPrefixPopup("f", hints, 200, 40)

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

func TestRenderPrefixPopup_TaskPrefixIncludesCommonActions(t *testing.T) {
	result := ansi.Strip(RenderPrefixPopup("t", PrefixHints{"t": TaskPrefixHints}, 200, 40))
	if result == "" {
		t.Fatal("expected task prefix popup to render")
	}

	for _, row := range []struct {
		description string
		detail      string
	}{
		{"new", "Create new task"},
		{"pin", "Toggle pin/unpin"},
		{"park", "Note where you left off"},
	} {
		assertPopupLineContains(t, result, row.description, row.detail)
	}
}

func TestTaskPrefixHints_MatchTaskPrefixGroupKeys(t *testing.T) {
	entries := keymap.TaskPrefixGroup(keymap.TaskPrefixDeps{})
	if len(entries) != 1 || entries[0].PrefixNode == nil {
		t.Fatalf("expected one task prefix node, got %#v", entries)
	}

	keymapKeys := map[string]bool{}
	for _, child := range entries[0].PrefixNode.Children {
		if child.Binding == nil {
			continue
		}
		keymapKeys[child.Binding.Key] = true
	}

	hintKeys := map[string]bool{}
	for _, hint := range TaskPrefixHints {
		hintKeys[hint.Key] = true
	}

	for key := range keymapKeys {
		if !hintKeys[key] {
			t.Errorf("TaskPrefixHints missing keymap child key %q", key)
		}
	}
	for key := range hintKeys {
		if !keymapKeys[key] {
			t.Errorf("TaskPrefixHints includes key %q without keymap child", key)
		}
	}
}

func TestBuildOutlinerTaskPrefixHints_CrossProjectDisablesOnlyCreationHints(t *testing.T) {
	hints := buildOutlinerTaskPrefixHints(true)
	disabledKeys := map[string]bool{}
	seenPark := false

	for _, hint := range hints {
		if hint.Key == "P" {
			seenPark = true
			if hint.Disabled {
				t.Fatal("cross-project outliner should keep P parking hint enabled")
			}
		}
		if hint.Disabled {
			disabledKeys[hint.Key] = true
		}
	}

	if !seenPark {
		t.Fatal("cross-project outliner task hints missing P parking hint")
	}
	for _, key := range []string{"n", "N"} {
		if !disabledKeys[key] {
			t.Errorf("cross-project outliner should disable task creation key %q", key)
		}
		delete(disabledKeys, key)
	}
	for key := range disabledKeys {
		t.Errorf("cross-project outliner disabled unexpected task key %q", key)
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
		"b": "waiting",
		"x": "cancel",
		"o": "reopen",
		"r": "review",
		"S": "paused",
		"m": "someday",
		"k": "skip",
		"D": "defer",
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
		"v": "focus",
		"p": "preview",
		"1": "depth-1",
		"2": "depth-2",
		"3": "depth-3",
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

func TestResolvePrefixCategoryHighlightSpan_DefaultUsesFirstRune(t *testing.T) {
	start, end, ok := resolvePrefixCategoryHighlightSpan("m", "move")
	if !ok {
		t.Fatal("expected a highlight span for default mnemonic")
	}
	if start != 0 || end != 1 {
		t.Fatalf("expected first-rune highlight span [0:1], got [%d:%d]", start, end)
	}
}

func TestResolvePrefixCategoryHighlightSpan_OverrideUsesMatchingMnemonic(t *testing.T) {
	category := "snooze"
	start, end, ok := resolvePrefixCategoryHighlightSpan("z", category)
	if !ok {
		t.Fatal("expected a highlight span for override mnemonic")
	}
	if got := category[start:end]; got != "z" {
		t.Fatalf("expected override to highlight %q, got %q", "z", got)
	}
}

func TestResolvePrefixCategoryHighlightSpan_OverrideFallsBackToFirstRune(t *testing.T) {
	category := "snooe"
	start, end, ok := resolvePrefixCategoryHighlightSpan("z", category)
	if !ok {
		t.Fatal("expected fallback highlight span when override mnemonic is missing")
	}
	if start != 0 || end != 1 {
		t.Fatalf("expected fallback first-rune span [0:1], got [%d:%d]", start, end)
	}
	if got := category[start:end]; got != "s" {
		t.Fatalf("expected fallback to highlight %q, got %q", "s", got)
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
