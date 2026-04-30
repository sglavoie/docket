package tui

func (r *KeybindingRegistry) registerRoadmapBindings() {
	views := []ViewType{ViewRoadmap}
	r.addForViews(views, []Keybinding{
		{Key: "Tab / shift+← / shift+→", Description: "Switch tab", Category: "Navigation"},
		{Key: "H/L", Description: "Horizontal scroll", Category: "Navigation"},
		{Key: "j/k", Description: "Move cursor up/down", Category: "Navigation"},
		{Key: "↑/↓", Description: "Move cursor up/down", Category: "Navigation"},
		{Key: "g/G", Description: "Jump to top/bottom", Category: "Navigation"},
		{Key: "ctrl+u/d", Description: "Half-page up/down", Category: "Navigation"},
		{Key: "z", Description: "Cycle zoom level (Week/Month/Quarter)", Category: "View"},
		{Key: "space", Description: "Collapse/expand row", Category: "View"},
		{Key: "enter", Description: "Go to task in project tree", Category: "Actions"},
		{Key: "o", Description: "Open task detail", Category: "Actions"},
		{Key: "s", Description: "Toggle scope (global/project)", Category: "View"},
		{Key: "t/T", Description: "Cycle zoom forward/backward", Category: "View"},
		{Key: "c", Description: "Toggle critical path highlight", Category: "View"},
		{Key: "r", Description: "Refresh roadmap data", Category: "Actions"},
		{Key: "?", Description: "Show help overlay", Category: "Navigation"},
	})
}
