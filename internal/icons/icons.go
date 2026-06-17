package icons

import "sync/atomic"

var useEmoji atomic.Bool

// SetUseEmoji configures whether emoji or Unicode symbols are returned by Get.
func SetUseEmoji(v bool) { useEmoji.Store(v) }

type iconPair struct{ emoji, unicode string }

var registry = map[string]iconPair{
	// Dashboard/Alert Indicators
	"overdue":   {"🔥", "‼"},
	"due_today": {"⏰", "⧖"},
	"blockers":  {"🚧", "⊜"},
	"inbox":     {"📥", "⊳"},
	"notes":     {"📝", "¶"},

	// Palette Action Icons
	"pinned":   {"📌", "⌃"},
	"date":     {"📅", "⊡"},
	"tag":      {"🏷", "#"},
	"settings": {"⚙️", "⚙"},
	"folder":   {"📂", "⌂"},
	"search":   {"🔍", "⌕"},
	"archive":  {"📦", "⊟"},
	"edit":     {"✏️", "✎"},
	"recur":    {"↺", "⟳"},

	// Type Hints (autocomplete)
	"idea":      {"💡", "∗"},
	"note_type": {"📝", "≡"},

	// Task Status Icons
	"status.todo":        {"📋", "□"},
	"status.in_progress": {"▶️", "▶"},
	"status.review":      {"🔎", "◑"},
	"status.standby":     {"🕒", "‖"},
	"status.maybe":       {"❓", "¿"},
	"status.done":        {"✅", "✓"},
	"status.blocked":     {"🚫", "⊘"},
	"status.canceled":    {"❌", "✕"},

	// Kanban Column Icons
	"kanban.todo":     {"📋", "○"},
	"kanban.review":   {"🔎", "◎"},
	"kanban.blocked":  {"🚫", "⊗"},
	"kanban.done":     {"✅", "✓"},
	"kanban.canceled": {"❌", "✗"},

	// Priority Icons
	"priority.1": {"🔴", "◉"},
	"priority.2": {"🟠", "●"},
	"priority.3": {"🔵", "◆"},
	"priority.4": {"⚪", "◇"},
	"priority.5": {"⬜", "·"},

	// Annotation/Meta Indicators
	"comment":      {"💬", "▪"},
	"decision":     {"⚖️", "▫"},
	"attachment":   {"📎", "⊕"},
	"github_issue": {"GH", "GH"},
	"checklist":    {"☑️", "☑"},
	"description":  {"📄", "≡"},

	// Checkbox States
	"checkbox.checked":   {"✅", "☑"},
	"checkbox.unchecked": {"⬜", "☐"},

	// Alert/Misc
	"warning": {"⚠️", "⚠"},
	"star":    {"⭐", "★"},
	"streak":  {"⚡", "↯"},
	"success": {"✅", "✓"},
	"error":   {"❌", "✕"},
	"close":   {"❌", "✕"},

	// Task Type Hints (autocomplete)
	"type.epic":  {"🏔️", "◆"},
	"type.story": {"📖", "◇"},
	"type.task":  {"📋", "□"},
	"type.bug":   {"🐛", "⚠"},
	"type.idea":  {"💡", "∗"},

	// Score/Blocked Indicators
	"score_bucket": {"💎", "◈"},
	"blocked_dot":  {"🚧", "◈"},

	// Project Status Icons
	"project.planning":    {"📐", "△"},
	"project.wip":         {"🚧", "⋙"},
	"project.active":      {"🟢", "▣"},
	"project.review":      {"🔍", "⊙"},
	"project.testing":     {"🧪", "⋔"},
	"project.stabilizing": {"🔒", "⊢"},
	"project.maintenance": {"🔧", "⚒"},
	"project.bugfix":      {"🐛", "⊠"},
	"project.paused":      {"💤", "⏏"},
	"project.archived":    {"📦", "⊔"},
}

// Get returns the icon string for the given key, using emoji or Unicode
// depending on the current setting.
func Get(key string) string {
	p, ok := registry[key]
	if !ok {
		return ""
	}
	if useEmoji.Load() {
		if p.emoji != "" {
			return p.emoji
		}
		return p.unicode
	}
	if p.unicode != "" {
		return p.unicode
	}
	return p.emoji
}
