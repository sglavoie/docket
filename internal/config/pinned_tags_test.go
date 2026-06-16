package config

import (
	"slices"
	"testing"
)

func TestConfigGlobalPinnedTagHelpersNormalizeValidateAndToggle(t *testing.T) {
	cfg := NewConfig()
	cfg.GlobalPinnedTags = []string{" backend ", "bad,tag", "backend"}

	if !cfg.IsGlobalTagPinned("#backend") {
		t.Fatal("IsGlobalTagPinned(#backend) = false, want true")
	}
	if got, want := cfg.GlobalPinnedTagList(), []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("GlobalPinnedTagList() = %v, want %v", got, want)
	}

	added, err := cfg.PinGlobalTag(" #urgent ")
	if err != nil {
		t.Fatalf("PinGlobalTag() error = %v", err)
	}
	if !added {
		t.Fatal("PinGlobalTag() added = false, want true")
	}
	if got, want := cfg.GlobalPinnedTags, []string{"backend", "urgent"}; !slices.Equal(got, want) {
		t.Fatalf("cfg.GlobalPinnedTags = %v, want %v", got, want)
	}

	added, err = cfg.PinGlobalTag("urgent")
	if err != nil {
		t.Fatalf("duplicate PinGlobalTag() error = %v", err)
	}
	if added {
		t.Fatal("duplicate PinGlobalTag() added = true, want false")
	}

	pinned, err := cfg.ToggleGlobalPinnedTag("urgent")
	if err != nil {
		t.Fatalf("ToggleGlobalPinnedTag(unpin) error = %v", err)
	}
	if pinned || cfg.IsGlobalTagPinned("urgent") {
		t.Fatal("ToggleGlobalPinnedTag(urgent) should unpin urgent")
	}

	pinned, err = cfg.ToggleGlobalPinnedTag("#frontend")
	if err != nil {
		t.Fatalf("ToggleGlobalPinnedTag(pin) error = %v", err)
	}
	if !pinned || !cfg.IsGlobalTagPinned("frontend") {
		t.Fatal("ToggleGlobalPinnedTag(frontend) should pin frontend")
	}

	if !cfg.UnpinGlobalTag("#backend") {
		t.Fatal("UnpinGlobalTag(#backend) = false, want true")
	}
	if cfg.IsGlobalTagPinned("backend") {
		t.Fatal("backend should not remain globally pinned")
	}

	if _, err := cfg.PinGlobalTag("bad,tag"); err == nil {
		t.Fatal("PinGlobalTag(bad,tag) error = nil, want error")
	}
}
