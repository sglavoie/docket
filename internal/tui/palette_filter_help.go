package tui

// buildHelpItems returns a list of filteredPaletteItems that serve as a syntax reference.
func (m *CommandPaletteModel) buildHelpItems() []filteredPaletteItem {
	helpEntries := []struct {
		name string
		desc string
	}{
		{"+Buy milk !2 #errand @tomorrow", "Inline task creation with priority, tag, due date"},
		{":cmd, :task, :prj, :tag, :sec, :note, :view", "Category prefix filters"},
		{":recent", "Show recently executed commands"},
		{"#123", "Jump directly to task by short ID"},
		{"\"exact phrase\"", "Quoted exact substring match"},
		{"-exclude", "Exclude items matching a term"},
		{"bug|fix", "OR search: match either term"},
		{"priority:1 or priority:1-3", "Filter tasks by priority value or range"},
		{"status:todo or status:todo|progress", "Filter tasks by status"},
		{"tag:name or tag:bug|feature", "Filter tasks by tag"},
		{"section:backlog", "Filter tasks by section"},
		{"@overdue @today @tomorrow @this-week", "Filter tasks by due date"},
		{"Ctrl+H", "Toggle completed/canceled tasks"},
		{"Ctrl+O", "Cycle sort: score → priority → due date"},
		{"Ctrl+P", "Toggle compact mode (hide preview)"},
		{"Ctrl+N", "Jump to next category section"},
		{"Ctrl+B", "Save current search as bookmark"},
		{"Ctrl+F", "Pin/unpin a command"},
		{"Tab", "Show actions for selected item"},
		{"Space", "Multi-select tasks for bulk actions"},
	}

	items := make([]filteredPaletteItem, len(helpEntries))
	for i, entry := range helpEntries {
		items[i] = filteredPaletteItem{
			Item: PaletteItem{
				Name:        entry.name,
				Description: entry.desc,
				Category:    ItemCategoryCommands,
			},
		}
	}
	return items
}
