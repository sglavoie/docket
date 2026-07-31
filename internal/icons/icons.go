package icons

var registry = map[string]string{
	"overdue": "‼", "due_today": "⧖", "inbox": "⊳", "notes": "¶",
	"pinned": "⌃", "date": "⊡", "tag": "#", "settings": "⚙",
	"folder": "⌂", "search": "⌕", "archive": "⊟", "edit": "✎", "recur": "⟳",
	"status.todo": "□", "status.in_progress": "▶", "status.review": "◑",
	"status.standby": "‖", "status.maybe": "¿", "status.done": "✓",
	"status.blocked": "⊘", "status.canceled": "✕",
	"priority.1": "◉", "priority.2": "●", "priority.3": "◆",
	"priority.4": "◇", "priority.5": "·",
	"comment": "▪", "decision": "▫", "attachment": "⊕", "github_issue": "GH",
	"checklist":        "☑",
	"checkbox.checked": "☑", "checkbox.unchecked": "☐",
	"warning": "⚠", "star": "★", "streak": "↯", "success": "✓",
	"error": "✕", "close": "✕",
}

func Get(key string) string { return registry[key] }
