package models

// ProjectStatusIcon represents a curated status icon for a project phase.
type ProjectStatusIcon struct {
	Key   string
	Emoji string
	Label string
}

var projectStatusIcons = []ProjectStatusIcon{
	{Key: "planning", Emoji: "📐", Label: "Planning"},
	{Key: "wip", Emoji: "🚧", Label: "Work in Progress"},
	{Key: "active", Emoji: "🟢", Label: "Active"},
	{Key: "review", Emoji: "🔍", Label: "In Review"},
	{Key: "testing", Emoji: "🧪", Label: "Testing"},
	{Key: "stabilizing", Emoji: "🔒", Label: "Stabilizing"},
	{Key: "maintenance", Emoji: "🔧", Label: "Maintenance"},
	{Key: "bugfix", Emoji: "🐛", Label: "Bugfixes Only"},
	{Key: "paused", Emoji: "💤", Label: "Paused"},
	{Key: "archived", Emoji: "📦", Label: "Archived"},
}

// AllProjectStatusIcons returns all available project status icons.
func AllProjectStatusIcons() []ProjectStatusIcon {
	return projectStatusIcons
}

// ProjectStatusIconByKey returns the icon for the given key, or nil if not found.
func ProjectStatusIconByKey(key string) *ProjectStatusIcon {
	for i := range projectStatusIcons {
		if projectStatusIcons[i].Key == key {
			return &projectStatusIcons[i]
		}
	}
	return nil
}
