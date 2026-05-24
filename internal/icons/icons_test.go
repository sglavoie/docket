package icons

import "testing"

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
		{"blockers", "⊜"},
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
		{"idea", "∗"},
		{"note_type", "≡"},
		{"status.todo", "□"},
		{"status.in_progress", "▶"},
		{"status.review", "◑"},
		{"status.standby", "‖"},
		{"status.maybe", "¿"},
		{"status.done", "✓"},
		{"status.blocked", "⊘"},
		{"status.canceled", "✕"},
		{"kanban.todo", "○"},
		{"kanban.review", "◎"},
		{"kanban.blocked", "⊗"},
		{"kanban.done", "✓"},
		{"kanban.canceled", "✗"},
		{"priority.1", "◉"},
		{"priority.2", "●"},
		{"priority.3", "◆"},
		{"priority.4", "◇"},
		{"priority.5", "·"},
		{"comment", "▪"},
		{"decision", "▫"},
		{"attachment", "⊕"},
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
		{"type.epic", "◆"},
		{"type.story", "◇"},
		{"type.task", "□"},
		{"type.bug", "⚠"},
		{"type.idea", "∗"},
		{"score_bucket", "◈"},
		{"blocked_dot", "◈"},
		{"project.planning", "△"},
		{"project.wip", "⋙"},
		{"project.active", "▣"},
		{"project.review", "⊙"},
		{"project.testing", "⋔"},
		{"project.stabilizing", "⊢"},
		{"project.maintenance", "⚒"},
		{"project.bugfix", "⊠"},
		{"project.paused", "⏏"},
		{"project.archived", "⊔"},
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
		{"blockers", "🚧"},
		{"inbox", "📥"},
		{"notes", "📝"},
		{"pinned", "📌"},
		{"date", "📅"},
		{"tag", "🏷"},
		{"folder", "📂"},
		{"search", "🔍"},
		{"archive", "📦"},
		{"edit", "✏"},
		{"recur", "↺"},
		{"idea", "💡"},
		{"note_type", "📝"},
		{"status.todo", "📋"},
		{"status.in_progress", "▶️"},
		{"status.review", "🔎"},
		{"status.standby", "⏸"},
		{"status.maybe", "❓"},
		{"status.done", "✅"},
		{"status.blocked", "🚫"},
		{"status.canceled", "❌"},
		{"kanban.todo", "📋"},
		{"kanban.review", "🔎"},
		{"kanban.blocked", "🚫"},
		{"kanban.done", "✅"},
		{"kanban.canceled", "❌"},
		{"priority.1", "🔴"},
		{"priority.2", "🟠"},
		{"priority.3", "🔵"},
		{"priority.4", "⚪"},
		{"priority.5", "⬜"},
		{"comment", "💬"},
		{"decision", "⚖️"},
		{"attachment", "📎"},
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
		{"type.epic", "🏔️"},
		{"type.story", "📖"},
		{"type.task", "📋"},
		{"type.bug", "🐛"},
		{"type.idea", "💡"},
		{"score_bucket", "💎"},
		{"blocked_dot", "🚧"},
		{"project.planning", "📐"},
		{"project.wip", "🚧"},
		{"project.active", "🟢"},
		{"project.review", "🔍"},
		{"project.testing", "🧪"},
		{"project.stabilizing", "🔒"},
		{"project.maintenance", "🔧"},
		{"project.bugfix", "🐛"},
		{"project.paused", "💤"},
		{"project.archived", "📦"},
	}
	for _, tt := range tests {
		if got := Get(tt.key); got != tt.want {
			t.Errorf("Get(%q) emoji = %q, want %q", tt.key, got, tt.want)
		}
	}
}
