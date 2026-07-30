package icons

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestStreakUnicodeFallbackDistinct(t *testing.T) {
	SetUseEmoji(false)
	unicode := Get("streak")
	SetUseEmoji(true)
	defer SetUseEmoji(false)
	emoji := Get("streak")
	if unicode == emoji {
		t.Errorf("streak unicode fallback %q must differ from emoji %q", unicode, emoji)
	}
}

func TestGetUnknownKey(t *testing.T) {
	if got := Get("nonexistent_key"); got != "" {
		t.Errorf("Get(unknown) = %q, want %q", got, "")
	}
}

func TestGetUnicodeDefault(t *testing.T) {
	SetUseEmoji(false)
	tests := []struct {
		key  string
		want string
	}{
		{"overdue", "‼"},
		{"due_today", "⧖"},
		{"inbox", "⊳"},
		{"notes", "¶"},
		{"pinned", "⌃"},
		{"date", "⊡"},
		{"tag", "#"},
		{"settings", "⚙"},
		{"folder", "⌂"},
		{"search", "⌕"},
		{"archive", "⊟"},
		{"edit", "✎"},
		{"recur", "⟳"},
		{"status.todo", "□"},
		{"status.in_progress", "▶"},
		{"status.review", "◑"},
		{"status.standby", "‖"},
		{"status.maybe", "¿"},
		{"status.done", "✓"},
		{"status.blocked", "⊘"},
		{"status.canceled", "✕"},
		{"priority.1", "◉"},
		{"priority.2", "●"},
		{"priority.3", "◆"},
		{"priority.4", "◇"},
		{"priority.5", "·"},
		{"comment", "▪"},
		{"decision", "▫"},
		{"attachment", "⊕"},
		{"github_issue", "GH"},
		{"checklist", "☑"},
		{"description", "≡"},
		{"checkbox.checked", "☑"},
		{"checkbox.unchecked", "☐"},
		{"warning", "⚠"},
		{"star", "★"},
		{"streak", "↯"},
		{"success", "✓"},
		{"error", "✕"},
		{"close", "✕"},
	}
	for _, tt := range tests {
		if got := Get(tt.key); got != tt.want {
			t.Errorf("Get(%q) unicode = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestGetEmojiMode(t *testing.T) {
	SetUseEmoji(true)
	defer SetUseEmoji(false)
	tests := []struct {
		key  string
		want string
	}{
		{"overdue", "🔥"},
		{"due_today", "⏰"},
		{"inbox", "📥"},
		{"notes", "📝"},
		{"pinned", "📌"},
		{"date", "📅"},
		{"tag", "🏷"},
		{"settings", "⚙️"},
		{"folder", "📂"},
		{"search", "🔍"},
		{"archive", "📦"},
		{"edit", "✏️"},
		{"recur", "↺"},
		{"status.todo", "📋"},
		{"status.in_progress", "▶️"},
		{"status.review", "🔎"},
		{"status.standby", "🕒"},
		{"status.maybe", "❓"},
		{"status.done", "✅"},
		{"status.blocked", "🚫"},
		{"status.canceled", "❌"},
		{"priority.1", "🔴"},
		{"priority.2", "🟠"},
		{"priority.3", "🔵"},
		{"priority.4", "⚪"},
		{"priority.5", "⬜"},
		{"comment", "💬"},
		{"decision", "⚖️"},
		{"attachment", "📎"},
		{"github_issue", "GH"},
		{"checklist", "☑️"},
		{"description", "📄"},
		{"checkbox.checked", "✅"},
		{"checkbox.unchecked", "⬜"},
		{"warning", "⚠️"},
		{"star", "⭐"},
		{"streak", "⚡"},
		{"success", "✅"},
		{"error", "❌"},
		{"close", "❌"},
	}
	for _, tt := range tests {
		if got := Get(tt.key); got != tt.want {
			t.Errorf("Get(%q) emoji = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestEditEmojiUsesEmojiPresentation(t *testing.T) {
	SetUseEmoji(true)
	defer SetUseEmoji(false)

	if got := Get("edit"); !strings.Contains(got, "\ufe0f") {
		t.Fatalf("Get(%q) emoji = %q, want emoji presentation selector", "edit", got)
	}
}

// TestColumnAlignedGroupsHaveConsistentWidth guards against width drift within
// icon groups that share a single column slot in TUI list rows. Mixed cell
// widths inside a group break PadBadge alignment (see CLAUDE.md TUI Column
// Alignment) because PadBadge sizes the column to max(width) and right-pads
// shorter badges, leaving content after the column visibly offset on rows
// whose icon is narrower. Width is measured with lipgloss.Width to match
// what PadBadge uses in production.
func TestColumnAlignedGroupsHaveConsistentWidth(t *testing.T) {
	groups := map[string][]string{
		"status": {
			"status.todo", "status.in_progress", "status.review",
			"status.standby", "status.maybe", "status.done",
			"status.blocked", "status.canceled",
		},
		"priority": {
			"priority.1", "priority.2", "priority.3",
			"priority.4", "priority.5",
		},
		"checkbox": {"checkbox.checked", "checkbox.unchecked"},
	}
	for _, mode := range []bool{false, true} {
		SetUseEmoji(mode)
		label := "unicode"
		if mode {
			label = "emoji"
		}
		for groupName, keys := range groups {
			widths := make(map[int][]string)
			for _, k := range keys {
				w := lipgloss.Width(Get(k))
				widths[w] = append(widths[w], k)
			}
			if len(widths) > 1 {
				var parts []string
				for w, ks := range widths {
					parts = append(parts, fmt.Sprintf("w=%d:%s", w, strings.Join(ks, ",")))
				}
				t.Errorf("%s group (%s mode) has inconsistent cell widths: %s",
					groupName, label, strings.Join(parts, " | "))
			}
		}
	}
	SetUseEmoji(false)
}

func TestGetFallsBackWhenPreferredVariantMissing(t *testing.T) {
	original, hadOriginal := registry["test.fallback"]
	defer func() {
		if hadOriginal {
			registry["test.fallback"] = original
		} else {
			delete(registry, "test.fallback")
		}
		SetUseEmoji(false)
	}()

	registry["test.fallback"] = iconPair{emoji: "E", unicode: ""}
	SetUseEmoji(false)
	if got := Get("test.fallback"); got != "E" {
		t.Fatalf("Get(unicode mode) fallback = %q, want %q", got, "E")
	}

	registry["test.fallback"] = iconPair{emoji: "", unicode: "U"}
	SetUseEmoji(true)
	if got := Get("test.fallback"); got != "U" {
		t.Fatalf("Get(emoji mode) fallback = %q, want %q", got, "U")
	}
}
