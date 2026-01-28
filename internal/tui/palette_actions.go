package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

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
	case ItemCategorySavedSearch:
		return m.getSavedSearchActions(item)
	default:
		return nil
	}
}

// getTaskActions returns inline actions for a task item.
func (m *CommandPaletteModel) getTaskActions(item PaletteItem) []PaletteAction {
	if item.Task == nil || item.Project == nil {
		return nil
	}
	task := item.Task
	project := item.Project

	actions := []PaletteAction{
		{
			Name:    "Go to task",
			Icon:    "→",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				return GlobalSearchSelectMsg{Task: task, Project: project}
			},
		},
		{
			Name:    "View",
			Icon:    "👁",
			KeyHint: "v",
			Handler: func() tea.Msg {
				return GlowViewMsg{Task: task}
			},
		},
		{
			Name:    "Edit",
			Icon:    "✏",
			KeyHint: "e",
			Handler: func() tea.Msg {
				return EditTaskMsg{Task: task}
			},
		},
	}

	// Add status-based actions
	if task.Status != models.StatusDone && task.Status != models.StatusCanceled {
		actions = append(actions, PaletteAction{
			Name:    "Complete",
			Icon:    "✓",
			KeyHint: "x",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "complete"}
			},
		})
	}

	if task.Status == models.StatusTodo {
		actions = append(actions, PaletteAction{
			Name:    "Start",
			Icon:    "▶",
			KeyHint: "s",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "start"}
			},
		})
	}

	// Block action for active, non-blocked tasks
	if task.Status.IsActive() && task.Status != models.StatusBlocked {
		actions = append(actions, PaletteAction{
			Name:    "Block",
			Icon:    "◈",
			KeyHint: "b",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "block"}
			},
		})
	}

	// Cancel action for active tasks
	if task.Status.IsActive() {
		actions = append(actions, PaletteAction{
			Name:    "Cancel",
			Icon:    "✕",
			KeyHint: "X",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "cancel"}
			},
		})
	}

	if task.Status == models.StatusDone || task.Status == models.StatusCanceled {
		actions = append(actions, PaletteAction{
			Name:    "Reopen",
			Icon:    "↺",
			KeyHint: "r",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "reopen"}
			},
		})
	}

	// Add due date action
	actions = append(actions, PaletteAction{
		Name:    "Set due date",
		Icon:    "📅",
		KeyHint: "d",
		Handler: func() tea.Msg {
			return SetDueDateMsg{Task: task, Project: project}
		},
	})

	// Add copy reference action
	actions = append(actions, PaletteAction{
		Name:    "Copy reference",
		Icon:    "📋",
		KeyHint: "c",
		Handler: func() tea.Msg {
			return CopyTaskReferenceMsg{Task: task}
		},
	})

	// Add move to section action
	actions = append(actions, PaletteAction{
		Name:    "Move to section",
		Icon:    "§",
		KeyHint: "m",
		Handler: func() tea.Msg {
			return ShowSectionPickerMsg{Task: task, Project: project}
		},
	})

	// Add set priority action
	actions = append(actions, PaletteAction{
		Name:    "Set priority",
		Icon:    "!",
		KeyHint: "p",
		Handler: func() tea.Msg {
			return ShowPriorityPickerMsg{Task: task, Project: project}
		},
	})

	// Add subtask action for non-leaf types
	if task.Type == models.TaskTypeEpic || task.Type == models.TaskTypeStory {
		actions = append(actions, PaletteAction{
			Name:    "Add subtask",
			Icon:    "+",
			KeyHint: "a",
			Handler: func() tea.Msg {
				return CreateTaskMsg{ParentTask: task}
			},
		})
	}

	return actions
}

// getProjectActions returns inline actions for a project item.
func (m *CommandPaletteModel) getProjectActions(item PaletteItem) []PaletteAction {
	if item.Project == nil {
		return nil
	}
	project := item.Project

	actions := []PaletteAction{
		{
			Name:    "Open",
			Icon:    "→",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				return SelectProjectMsg{Project: project}
			},
		},
		{
			Name:    "Edit",
			Icon:    "✏",
			KeyHint: "e",
			Handler: func() tea.Msg {
				return EditProjectMsg{Project: project}
			},
		},
	}

	if project.Archived {
		actions = append(actions, PaletteAction{
			Name:    "Restore",
			Icon:    "↺",
			KeyHint: "r",
			Handler: func() tea.Msg {
				project.Unarchive()
				return ProjectRestoredMsg{Project: project}
			},
		})
	} else {
		actions = append(actions, PaletteAction{
			Name:    "Archive",
			Icon:    "📦",
			KeyHint: "x",
			Handler: func() tea.Msg {
				project.Archive()
				return ProjectArchivedMsg{Project: project}
			},
		})
	}

	// Pin/unpin action
	pinName := "Pin"
	pinIcon := "📌"
	if project.Pinned {
		pinName = "Unpin"
		pinIcon = "📍"
	}
	actions = append(actions, PaletteAction{
		Name:    pinName,
		Icon:    pinIcon,
		KeyHint: "p",
		Handler: func() tea.Msg {
			project.TogglePinned()
			return ProjectPinnedToggledMsg{Project: project}
		},
	})

	return actions
}

// getBulkActionsForSelectedTasks returns actions that can be applied to multiple selected tasks.
func (m *CommandPaletteModel) getBulkActionsForSelectedTasks() []PaletteAction {
	if len(m.selectedTasks) == 0 {
		return nil
	}

	// Collect the selected tasks and their projects
	var selectedItems []struct {
		Task    *models.Task
		Project *models.Project
	}
	for _, filtered := range m.filteredItems {
		item := filtered.Item
		if item.Category == ItemCategoryTasks && item.Task != nil && item.Project != nil {
			if m.selectedTasks[item.Task.ID] {
				selectedItems = append(selectedItems, struct {
					Task    *models.Task
					Project *models.Project
				}{Task: item.Task, Project: item.Project})
			}
		}
	}

	if len(selectedItems) == 0 {
		return nil
	}

	// Check what actions are applicable to all selected tasks
	canComplete := true
	canStart := true
	for _, item := range selectedItems {
		if item.Task.Status == models.StatusDone || item.Task.Status == models.StatusCanceled {
			canComplete = false
		}
		if item.Task.Status != models.StatusTodo {
			canStart = false
		}
	}

	var actions []PaletteAction

	// Complete all
	if canComplete {
		items := selectedItems // Capture for closure
		actions = append(actions, PaletteAction{
			Name:    fmt.Sprintf("Complete all (%d)", len(items)),
			Icon:    "✓",
			KeyHint: "x",
			Handler: func() tea.Msg {
				return CommandPaletteBulkActionMsg{
					Action: "complete",
					Tasks:  items,
				}
			},
		})
	}

	// Start all
	if canStart {
		items := selectedItems
		actions = append(actions, PaletteAction{
			Name:    fmt.Sprintf("Start all (%d)", len(items)),
			Icon:    "▶",
			KeyHint: "s",
			Handler: func() tea.Msg {
				return CommandPaletteBulkActionMsg{
					Action: "start",
					Tasks:  items,
				}
			},
		})
	}

	// Convert to PaletteBulkTaskItem for the new message types
	bulkItems := make([]PaletteBulkTaskItem, len(selectedItems))
	for i, item := range selectedItems {
		bulkItems[i] = PaletteBulkTaskItem{Task: item.Task, Project: item.Project}
	}

	// Set priority for all
	actions = append(actions, PaletteAction{
		Name:    fmt.Sprintf("Set priority (%d)", len(bulkItems)),
		Icon:    "!",
		KeyHint: "p",
		Handler: func() tea.Msg {
			return PaletteBulkPriorityMsg{Tasks: bulkItems}
		},
	})

	// Add tag to all
	actions = append(actions, PaletteAction{
		Name:    fmt.Sprintf("Add tag (%d)", len(bulkItems)),
		Icon:    "#",
		KeyHint: "t",
		Handler: func() tea.Msg {
			return PaletteBulkTagMsg{Tasks: bulkItems}
		},
	})

	// Set due date for all
	actions = append(actions, PaletteAction{
		Name:    fmt.Sprintf("Set due date (%d)", len(bulkItems)),
		Icon:    "📅",
		KeyHint: "d",
		Handler: func() tea.Msg {
			return PaletteBulkDueDateMsg{Tasks: bulkItems}
		},
	})

	// Clear selection
	actions = append(actions, PaletteAction{
		Name:    "Clear selection",
		Icon:    "✕",
		KeyHint: "c",
		Handler: func() tea.Msg {
			return CommandPaletteClearSelectionMsg{}
		},
	})

	return actions
}

// getTagActions returns inline actions for a tag item.
func (m *CommandPaletteModel) getTagActions(item PaletteItem) []PaletteAction {
	if item.Tag == "" {
		return nil
	}
	tag := item.Tag
	project := m.findProjectWithTag(tag)

	actions := []PaletteAction{
		{
			Name:    "Filter by tag",
			Icon:    "🏷",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				if project != nil {
					return GlobalTagNavigateMsg{Project: project, Tag: tag}
				}
				return nil
			},
		},
		{
			Name:    "Manage tags",
			Icon:    "⚙",
			KeyHint: "t",
			Handler: func() tea.Msg {
				return ShowGlobalTagManagerMsg{}
			},
		},
	}

	return actions
}

// getSectionActions returns inline actions for a section item.
func (m *CommandPaletteModel) getSectionActions(item PaletteItem) []PaletteAction {
	if item.Section == "" {
		return nil
	}
	section := item.Section
	project := m.findProjectWithSection(section)

	actions := []PaletteAction{
		{
			Name:    "Filter by section",
			Icon:    "📂",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				if project != nil {
					return CommandPaletteSectionSelectMsg{Section: section, Project: project}
				}
				return nil
			},
		},
	}

	return actions
}

// getSavedSearchActions returns inline actions for a saved search item.
func (m *CommandPaletteModel) getSavedSearchActions(item PaletteItem) []PaletteAction {
	if item.SavedSearch == "" {
		return nil
	}
	name := item.Name
	query := item.SavedSearch

	actions := []PaletteAction{
		{
			Name:    "Run search",
			Icon:    "🔍",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				return ExecuteSavedSearchMsg{Query: query}
			},
		},
		{
			Name:    "Delete",
			Icon:    "✕",
			KeyHint: "d",
			Handler: func() tea.Msg {
				return DeleteSavedSearchMsg{Name: name}
			},
		},
	}

	return actions
}
