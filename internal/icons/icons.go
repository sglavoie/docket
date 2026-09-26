package icons

var registry = map[string]string{
	"date":         "⊡",
	"status.inbox": "◌", "status.backlog": "□", "status.doing": "▶",
	"status.waiting": "⊘", "status.paused": "‖", "status.someday": "¿",
	"status.done": "✓", "status.canceled": "✕",
	"priority.1": "◉", "priority.2": "●", "priority.3": "◆",
	"github_issue":     "GH",
	"checkbox.checked": "☑", "checkbox.unchecked": "☐",
	"warning": "⚠", "success": "✓", "error": "✕",
}

func Get(key string) string { return registry[key] }
