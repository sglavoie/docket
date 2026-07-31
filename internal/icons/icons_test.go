package icons

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestGetUsesStableUnicodeRegistry(t *testing.T) {
	for key, want := range map[string]string{
		"overdue": "‼", "status.todo": "□", "priority.1": "◉",
		"attachment": "⊕", "checkbox.checked": "☑", "streak": "↯",
	} {
		if got := Get(key); got != want {
			t.Errorf("Get(%q) = %q, want %q", key, got, want)
		}
	}
	if got := Get("nonexistent_key"); got != "" {
		t.Errorf("Get(unknown) = %q", got)
	}
}

func TestColumnAlignedGroupsHaveConsistentWidth(t *testing.T) {
	for group, keys := range map[string][]string{
		"status":   {"status.todo", "status.in_progress", "status.review", "status.standby", "status.maybe", "status.done", "status.blocked", "status.canceled"},
		"priority": {"priority.1", "priority.2", "priority.3", "priority.4", "priority.5"},
		"checkbox": {"checkbox.checked", "checkbox.unchecked"},
	} {
		width := lipgloss.Width(Get(keys[0]))
		for _, key := range keys[1:] {
			if got := lipgloss.Width(Get(key)); got != width {
				t.Errorf("%s icon %q width = %d, want %d", group, key, got, width)
			}
		}
	}
}
