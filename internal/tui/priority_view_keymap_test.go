package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPriorityViewTopLevelPClosesView(t *testing.T) {
	project := newTestPriorityProject(t)
	m := NewPriorityViewModel(project, nil, nil)

	model, cmd := m.handleKeypress(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if model != m {
		t.Fatalf("handleKeypress(p) returned a different model: got %T, want *PriorityViewModel", model)
	}
	if cmd == nil {
		t.Fatal("handleKeypress(p) returned nil cmd; expected ClosePriorityViewMsg producer")
	}

	msg := cmd()
	if _, ok := msg.(ClosePriorityViewMsg); !ok {
		t.Fatalf("expected ClosePriorityViewMsg from top-level `p`, got %T", msg)
	}
}

func TestPriorityViewFpStillTogglesPreview(t *testing.T) {
	project := newTestPriorityProject(t)
	m := NewPriorityViewModel(project, nil, nil)

	if m.previewMode {
		t.Fatal("previewMode should default to false")
	}

	if _, _ = m.handleKeypress(tea.KeyPressMsg{Code: 'f', Text: "f"}); m.previewMode {
		t.Fatal("previewMode flipped after only `f`; expected the prefix to await a follow-up key")
	}
	m.handleKeypress(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if !m.previewMode {
		t.Fatal("expected `fp` prefix sequence to toggle previewMode on")
	}

	m.handleKeypress(tea.KeyPressMsg{Code: 'f', Text: "f"})
	m.handleKeypress(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if m.previewMode {
		t.Fatal("expected `fp` again to toggle previewMode off")
	}
}

func TestPriorityViewHandledKeysIncludesTopLevelP(t *testing.T) {
	m := &PriorityViewModel{}
	if !slices.Contains(m.HandledKeys(), "p") {
		t.Fatal("HandledKeys should include the top-level `p` close binding")
	}
}

func TestPriorityViewRegistryIncludesClosePriorityView(t *testing.T) {
	items := GetKeybindingRegistry().GetForView(ViewPriorityView)
	for _, item := range items {
		if item.Key == "p" && item.Description == "Close Priority view" {
			return
		}
	}
	t.Fatal("keybinding registry for ViewPriorityView should advertise `p: Close Priority view`")
}

func TestCrossOutlinerRegistryAdvertisesTogglePriorityMode(t *testing.T) {
	items := GetKeybindingRegistry().GetForView(ViewCrossProjectOutliner)
	for _, item := range items {
		if item.Key == "p" && item.Description == "Toggle Priority mode" {
			return
		}
	}
	t.Fatal("keybinding registry for ViewCrossProjectOutliner should advertise `p: Toggle Priority mode`")
}

func TestSingleProjectOutlinerRegistryKeepsPriorityViewLabel(t *testing.T) {
	items := GetKeybindingRegistry().GetForView(ViewOutliner)
	for _, item := range items {
		if item.Key == "p" && item.Description == "Priority view" {
			return
		}
	}
	t.Fatal("keybinding registry for ViewOutliner should keep `p: Priority view` label for the single-project context")
}
