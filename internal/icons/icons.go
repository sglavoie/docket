package icons

var registry = map[string]string{
	"overdue": "‼", "due_today": "⧖", "inbox": "⊳", "notes": "¶",
	"pinned": "⌃", "date": "⊡", "tag": "#", "settings": "⚙",
	"folder": "⌂", "search": "⌕", "archive": "⊟", "edit": "✎",
	"status.inbox": "□", "status.backlog": "□", "status.doing": "▶",
	"status.waiting": "⊘", "status.paused": "‖", "status.someday": "¿",
	"status.done": "✓", "status.canceled": "✕",
	"priority.1": "◉", "priority.2": "●", "priority.3": "◆",
	"comment": "▪", "attachment": "⊕", "github_issue": "GH",
	"checklist":        "☑",
	"checkbox.checked": "☑", "checkbox.unchecked": "☐",
	"warning": "⚠", "star": "★", "streak": "↯", "success": "✓",
	"error": "✕", "close": "✕",
}

func Get(key string) string { return registry[key] }
