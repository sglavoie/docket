package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

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
				return ShowTaskDetailMsg{Task: task, Project: project}
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
			Icon:    IconScrollRight,
			KeyHint: "s",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "start"}
			},
		})
	}

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

	if task.Status == models.StatusInProgress {
		actions = append(actions, PaletteAction{
			Name:    "Review",
			Icon:    "◑",
			KeyHint: "R",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "review"}
			},
		})
	}

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

	actions = append(actions,
		PaletteAction{
			Name:    "Set due date",
			Icon:    "📅",
			KeyHint: "d",
			Handler: func() tea.Msg {
				return SetDueDateMsg{Task: task, Project: project}
			},
		},
		PaletteAction{
			Name:    "Copy reference",
			Icon:    "📋",
			KeyHint: "c",
			Handler: func() tea.Msg {
				return CopyTaskReferenceMsg{Task: task}
			},
		},
		PaletteAction{
			Name:    "Move to section",
			Icon:    "§",
			KeyHint: "m",
			Handler: func() tea.Msg {
				return ShowSectionPickerMsg{Task: task, Project: project}
			},
		},
		PaletteAction{
			Name:    "Set priority",
			Icon:    "!",
			KeyHint: "p",
			Handler: func() tea.Msg {
				return ShowPriorityPickerMsg{Task: task, Project: project}
			},
		},
	)

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

// getBulkActionsForSelectedTasks returns actions that can be applied to multiple selected tasks.
func (m *CommandPaletteModel) getBulkActionsForSelectedTasks() []PaletteAction {
	if len(m.selectedTasks) == 0 {
		return nil
	}

	var selectedItems []struct {
		Task    *models.Task
		Project *models.Project
	}
	for _, filtered := range m.filteredItems {
		item := filtered.Item
		if item.Category == ItemCategoryTasks && item.Task != nil && item.Project != nil && m.selectedTasks[item.Task.ID] {
			selectedItems = append(selectedItems, struct {
				Task    *models.Task
				Project *models.Project
			}{Task: item.Task, Project: item.Project})
		}
	}

	if len(selectedItems) == 0 {
		return nil
	}

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

	if canComplete {
		items := selectedItems
		actions = append(actions, PaletteAction{
			Name:    fmt.Sprintf("Complete all (%d)", len(items)),
			Icon:    "✓",
			KeyHint: "x",
			Handler: func() tea.Msg {
				return CommandPaletteBulkActionMsg{Action: "complete", Tasks: items}
			},
		})
	}

	if canStart {
		items := selectedItems
		actions = append(actions, PaletteAction{
			Name:    fmt.Sprintf("Start all (%d)", len(items)),
			Icon:    IconScrollRight,
			KeyHint: "s",
			Handler: func() tea.Msg {
				return CommandPaletteBulkActionMsg{Action: "start", Tasks: items}
			},
		})
	}

	bulkItems := make([]PaletteBulkTaskItem, len(selectedItems))
	for i, item := range selectedItems {
		bulkItems[i] = PaletteBulkTaskItem{Task: item.Task, Project: item.Project}
	}

	actions = append(actions,
		PaletteAction{
			Name:    fmt.Sprintf("Set priority (%d)", len(bulkItems)),
			Icon:    "!",
			KeyHint: "p",
			Handler: func() tea.Msg {
				return PaletteBulkPriorityMsg{Tasks: bulkItems}
			},
		},
		PaletteAction{
			Name:    fmt.Sprintf("Add tag (%d)", len(bulkItems)),
			Icon:    "#",
			KeyHint: "t",
			Handler: func() tea.Msg {
				return PaletteBulkTagMsg{Tasks: bulkItems}
			},
		},
		PaletteAction{
			Name:    fmt.Sprintf("Set due date (%d)", len(bulkItems)),
			Icon:    "📅",
			KeyHint: "d",
			Handler: func() tea.Msg {
				return PaletteBulkDueDateMsg{Tasks: bulkItems}
			},
		},
		PaletteAction{
			Name:    "Clear selection",
			Icon:    "✕",
			KeyHint: "c",
			Handler: func() tea.Msg {
				return CommandPaletteClearSelectionMsg{}
			},
		},
	)

	return actions
}
