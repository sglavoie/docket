package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/icons"
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
			Icon:    icons.Get("edit"),
			KeyHint: "e",
			Handler: func() tea.Msg {
				return EditTaskMsg{Task: task}
			},
		},
	}

	if !task.IsTerminalState() {
		actions = append(actions, PaletteAction{
			Name:    "Complete",
			Icon:    icons.Get("status.done"),
			KeyHint: "x",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "complete"}
			},
		})
	}

	if taskInPlanningState(task) {
		actions = append(actions, PaletteAction{
			Name:    "Start",
			Icon:    IconScrollRight,
			KeyHint: "s",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "start"}
			},
		})
	}

	if !task.IsTerminalState() && !task.IsInState(models.StateWaiting) {
		actions = append(actions, PaletteAction{
			Name:    "Block",
			Icon:    icons.Get("blocked_dot"),
			KeyHint: "b",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "block"}
			},
		})
	}

	if task.IsInState(models.StateInProgress) {
		actions = append(actions, PaletteAction{
			Name:    "Review",
			Icon:    icons.Get("status.review"),
			KeyHint: "R",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "review"}
			},
		})
	}

	if !task.IsTerminalState() {
		actions = append(actions, PaletteAction{
			Name:    "Cancel",
			Icon:    icons.Get("status.canceled"),
			KeyHint: "X",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "cancel"}
			},
		})
	}

	if task.IsTerminalState() {
		actions = append(actions, PaletteAction{
			Name:    "Reopen",
			Icon:    icons.Get("recur"),
			KeyHint: "r",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "reopen"}
			},
		})
	}

	actions = append(actions,
		PaletteAction{
			Name:    "Set due date",
			Icon:    icons.Get("date"),
			KeyHint: "d",
			Handler: func() tea.Msg {
				return SetDueDateMsg{Task: task, Project: project}
			},
		},
		PaletteAction{
			Name:    "Copy reference",
			Icon:    IconDecision(),
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

	var selectedItems []TaskProjectRef
	for _, filtered := range m.filteredItems {
		item := filtered.Item
		if item.Category == ItemCategoryTasks && item.Task != nil && item.Project != nil && m.selectedTasks[item.Task.ID] {
			selectedItems = append(selectedItems, TaskProjectRef{Task: item.Task, Project: item.Project})
		}
	}

	if len(selectedItems) == 0 {
		return nil
	}

	canComplete := true
	canStart := true
	for _, item := range selectedItems {
		if item.Task.IsTerminalState() {
			canComplete = false
		}
		if !taskInPlanningState(item.Task) {
			canStart = false
		}
	}

	var actions []PaletteAction

	if canComplete {
		items := selectedItems
		actions = append(actions, PaletteAction{
			Name:    fmt.Sprintf("Complete all (%d)", len(items)),
			Icon:    icons.Get("status.done"),
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

	bulkItems := selectedItems

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
			Icon:    icons.Get("close"),
			KeyHint: "c",
			Handler: func() tea.Msg {
				return CommandPaletteClearSelectionMsg{}
			},
		},
	)

	return actions
}
