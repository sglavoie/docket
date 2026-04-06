package tui

func (r *KeybindingRegistry) registerMetricsDashboardBindings() {
	views := []ViewType{ViewMetricsDashboard}
	r.addForViews(views, []Keybinding{
		{Key: "[/]", Description: "Switch metric tab", Category: "Navigation"},
		{Key: "h/l", Description: "Switch metric tab", Category: "Navigation"},
		{Key: "t/T", Description: "Cycle time range forward/backward", Category: "Navigation"},
		{Key: "g", Description: "Toggle scope (global/project)", Category: "Navigation"},
		{Key: "r", Description: "Refresh metrics", Category: "Actions"},
		{Key: "j/k", Description: "Scroll blocked list (blocked tab)", Category: "Navigation"},
		{Key: "↑/↓", Description: "Scroll blocked list (blocked tab)", Category: "Navigation"},
		{Key: "enter", Description: "Go to task (blocked tab)", Category: "Actions"},
	})
}
