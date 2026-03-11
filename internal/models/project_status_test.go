package models

import "testing"

func TestAllProjectStatusIcons(t *testing.T) {
	icons := AllProjectStatusIcons()

	if len(icons) == 0 {
		t.Fatal("expected non-empty slice of project status icons")
	}

	// Verify a few known entries exist
	knownKeys := map[string]bool{
		"planning": false,
		"active":   false,
		"archived": false,
	}

	for _, icon := range icons {
		if _, ok := knownKeys[icon.Key]; ok {
			knownKeys[icon.Key] = true
		}
	}

	for key, found := range knownKeys {
		if !found {
			t.Errorf("expected to find icon with key %q", key)
		}
	}
}

func TestProjectStatusIconByKey(t *testing.T) {
	icon := ProjectStatusIconByKey("active")
	if icon == nil {
		t.Fatal("expected non-nil icon for key 'active'")
	}
	if icon.Key != "active" {
		t.Errorf("expected key 'active', got %q", icon.Key)
	}
	if icon.Emoji == "" {
		t.Error("expected non-empty emoji for 'active'")
	}
	if icon.Label == "" {
		t.Error("expected non-empty label for 'active'")
	}
}

func TestProjectStatusIconByKeyNonexistent(t *testing.T) {
	icon := ProjectStatusIconByKey("nonexistent")
	if icon != nil {
		t.Errorf("expected nil for nonexistent key, got %+v", icon)
	}
}
