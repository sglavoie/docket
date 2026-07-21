package tui

// getActionsForItem returns the available inline actions for a palette item.
// Commands have no inline actions (Enter executes them directly).
func (m *CommandPaletteModel) getActionsForItem(item PaletteItem) []PaletteAction {
	switch item.Category {
	case ItemCategoryTasks:
		return m.getTaskActions(item)
	case ItemCategoryProjects:
		return m.getProjectActions(item)
	case ItemCategoryTags:
		return m.getTagActions(item)
	case ItemCategorySections:
		return m.getSectionActions(item)
	case ItemCategorySavedFilter:
		return m.getSavedFilterActions(item)
	case ItemCategoryNotes:
		return m.getNoteActions(item)
	case ItemCategoryViews:
		return m.getViewActions(item)
	default:
		return nil
	}
}
