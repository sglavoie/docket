package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// findProjectWithTag returns the first project that contains the specified tag.
func (m *CommandPaletteModel) findProjectWithTag(tag string) *models.Project {
	for _, item := range m.allItems {
		if item.Category == ItemCategoryTasks && item.Task != nil {
			if slices.Contains(item.Task.Tags, tag) {
				return item.Project
			}
		}
	}
	return nil
}

// findProjectWithSection returns the first project that contains the specified section.
func (m *CommandPaletteModel) findProjectWithSection(section string) *models.Project {
	for _, item := range m.allItems {
		if item.Category == ItemCategoryTasks && item.Task != nil && item.Task.InSection(section) {
			return item.Project
		}
	}
	return nil
}

// executeItem handles selection of an item from the palette.
// For commands, it invokes the handler directly and returns its message.
// For tasks, it emits PaletteEntitySelectMsg to navigate to the task.
// For projects, it emits SelectProjectMsg to switch to that project.
// For tags and sections, it emits navigation messages with the first associated project.
// All item types use CommandPaletteExecuteMsg to ensure the palette closes and action
// executes in the same update cycle.
// The selected item is recorded for recent items boosting.
func (m *CommandPaletteModel) executeItem(item PaletteItem) (tea.Model, tea.Cmd) {
	itemID := m.getItemID(item)
	if itemID != "" {
		recordRecentSelection(item, itemID)
		m.persistRecentSelections()
	}

	switch item.Category {
	case ItemCategoryCommands:
		if item.Command != nil && item.Command.Handler != nil {
			result := item.Command.Handler()
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: result}
			}
		}
	case ItemCategoryTasks:
		if item.Task != nil && item.Project != nil {
			task := item.Task
			project := item.Project
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: PaletteEntitySelectMsg{Task: task, Project: project}}
			}
		}
	case ItemCategoryProjects:
		if item.Project != nil {
			project := item.Project
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: SelectProjectMsg{Project: project}}
			}
		}
	case ItemCategoryTags:
		if item.Tag != "" {
			tag := item.Tag
			project := m.findProjectWithTag(tag)
			if project != nil {
				return m, func() tea.Msg {
					return CommandPaletteExecuteMsg{ResultMsg: GlobalTagNavigateMsg{Project: project, Tag: tag}}
				}
			}
		}
	case ItemCategorySections:
		if item.Section != "" {
			section := item.Section
			project := m.findProjectWithSection(section)
			if project != nil {
				return m, func() tea.Msg {
					return CommandPaletteExecuteMsg{ResultMsg: CommandPaletteSectionSelectMsg{Section: section, Project: project}}
				}
			}
		}
	case ItemCategorySavedFilter:
		if item.SavedFilter != nil {
			sf := *item.SavedFilter
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: LoadSavedFilterMsg{Filter: sf}}
			}
		}
	case ItemCategoryNotes:
		if item.Project != nil {
			project := item.Project
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: ShowNotesMsg{Project: project}}
			}
		}
	case ItemCategoryViews:
		if item.AliasTarget != "" {
			target := item.AliasTarget
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: NavigatePaletteAliasMsg{Target: target}}
			}
		}
		viewType := item.ViewTarget
		return m, func() tea.Msg {
			return CommandPaletteExecuteMsg{ResultMsg: NavigateToViewMsg{PresentationID: viewType}}
		}
	}

	return m, nil
}

// peekItem opens a task Peek above the palette without discarding the palette
// or its query. A following Esc therefore returns to the exact search result.
func (m *CommandPaletteModel) peekItem(item PaletteItem) (tea.Model, tea.Cmd) {
	if item.Category != ItemCategoryTasks || item.Task == nil || item.Project == nil {
		return m, nil
	}
	itemID := m.getItemID(item)
	if itemID != "" {
		recordRecentSelection(item, itemID)
		m.persistRecentSelections()
	}
	task := item.Task
	return m, func() tea.Msg {
		return CommandPaletteExecuteMsg{ResultMsg: GlowViewMsg{Task: task}}
	}
}
