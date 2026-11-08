package config

import (
	"slices"
	"testing"
)

func TestWithPaletteUseMovesToFrontAndBounds(t *testing.T) {
	state := MachineState{}
	for _, id := range []string{"a", "b", "c", "a"} {
		state, _ = state.WithPaletteUse(id)
	}
	if want := []string{"a", "c", "b"}; !slices.Equal(state.PaletteRecent, want) {
		t.Fatalf("recent = %v, want %v", state.PaletteRecent, want)
	}
	if _, moved := state.WithPaletteUse("a"); moved {
		t.Fatal("re-running the newest action rewrote the list")
	}
	for i := range PaletteRecentLimit + 3 {
		state, _ = state.WithPaletteUse(string(rune('d' + i)))
	}
	if len(state.PaletteRecent) != PaletteRecentLimit {
		t.Fatalf("recent holds %d, want %d", len(state.PaletteRecent), PaletteRecentLimit)
	}
	if err := state.Validate(); err != nil {
		t.Fatalf("a bounded list failed validation: %v", err)
	}
	state.PaletteRecent = []string{"x", "x"}
	if err := state.Validate(); err == nil {
		t.Fatal("a duplicate recent id validated")
	}
}

func TestMachineThemeAcceptsTheCatalogAlone(t *testing.T) {
	for _, name := range MachineThemes {
		state := MachineState{Theme: name}
		if err := state.Validate(); err != nil {
			t.Fatalf("theme %q refused: %v", name, err)
		}
	}
	if err := (&MachineState{Theme: "neon"}).Validate(); err == nil {
		t.Fatal("an unknown theme validated")
	}
}
