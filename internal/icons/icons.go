package icons

import "sync/atomic"

var useEmoji atomic.Bool

// SetUseEmoji configures whether emoji or Unicode symbols are returned by Get.
func SetUseEmoji(v bool) { useEmoji.Store(v) }

type iconPair struct{ emoji, unicode string }

var registry = map[string]iconPair{
	// Due-date and workflow indicators
	"overdue":   {"🔥", "‼"},
	"due_today": {"⏰", "⧖"},
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

	// Task Status Icons
	"status.todo":        {"📋", "□"},
	"status.in_progress": {"▶️", "▶"},
	"status.review":      {"🔎", "◑"},
	"status.standby":     {"🕒", "‖"},
	"status.maybe":       {"❓", "¿"},
	"status.done":        {"✅", "✓"},
	"status.blocked":     {"🚫", "⊘"},
	"status.canceled":    {"❌", "✕"},

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

	// General feedback
	"warning": {"⚠️", "⚠"},
	"star":    {"⭐", "★"},
	"streak":  {"⚡", "↯"},
	"success": {"✅", "✓"},
	"error":   {"❌", "✕"},
	"close":   {"❌", "✕"},
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
