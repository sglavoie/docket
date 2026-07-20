package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
)

func (m *AppModel) taskCommands() []Command {
	return []Command{
		{
			Name:        "quick capture",
			Description: "Quick capture a single task",
			Shortcut:    ";i",
			Category:    CategoryTasks,
			Aliases:     []string{"capture", "inbox"},
			Handler: func() tea.Msg {
				if m.nonPaletteOrigin() == ViewCrossProjectOutliner && m.crossOutliner != nil {
					project := m.crossOutliner.cursorProject()
					if project == nil {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return ShowQuickCaptureMsg{Project: project, AllProjects: m.activeProjects()}
				}
				return m.resolveQuickCaptureProject()
			},
		},
		{
			Name:        "bulk capture",
			Description: "Bulk capture multiple tasks",
			Shortcut:    "I",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"multi capture", "batch add"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return StatusMsg("No project selected")
				}
				return ShowBulkCaptureMsg{Project: project}
			},
		},
		{
			Name:        "global bulk capture",
			Description: "Bulk capture multiple tasks from any view",
			Shortcut:    ";I",
			Category:    CategoryTasks,
			Aliases:     []string{"global multi capture", "global batch add"},
			Handler: func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeBulkCapture}
			},
		},
		{
			Name:        "global note create",
			Description: "Create a new note in any project from any view",
			Shortcut:    ";{",
			Category:    CategoryNotes,
			Aliases:     []string{"global note", "new note"},
			Handler: func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeNoteCreate}
			},
		},
		{
			Name:        "edit last task",
			Description: "Edit the most recently modified task",
			Shortcut:    "ctrl+l",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"edit recent", "last modified", "recent task"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return StatusMsg("No project selected")
				}
				recentTasks := storage.GetRecentTasks(project.Tasks, 1)
				if len(recentTasks) == 0 {
					return StatusMsg("No recent tasks")
				}
				return EditTaskMsg{Task: recentTasks[0], Project: project}
			},
		},
		{
			Name:        "archive completed tasks",
			Description: "Move completed tasks to archive file",
			Category:    CategoryActions,
			Views:       []ViewType{ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"archive", "archive done", "archive completed"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return ArchiveCompletedTasksMsg{}
				}
				return ArchiveCompletedTasksMsg{Project: project}
			},
		},
		{
			Name:        "archive all completed tasks",
			Description: "Archive completed tasks across all projects",
			Category:    CategoryActions,
			Aliases:     []string{"archive all", "archive everything"},
			Handler: func() tea.Msg {
				return ArchiveAllCompletedTasksMsg{}
			},
		},
		{
			Name:        "global status update",
			Description: "Update task status from any view",
			Shortcut:    ";,",
			Category:    CategoryStatus,
			Aliases:     []string{"set status", "change status", "update status"},
			Handler: func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeStatusUpdate}
			},
		},
		{
			Name:        "global priority update",
			Description: "Update task priority from any view",
			Shortcut:    ";!",
			Category:    CategoryPriority,
			Aliases:     []string{"set priority", "change priority", "update priority"},
			Handler: func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposePriorityUpdate}
			},
		},
		{
			Name:        "global due date update",
			Description: "Update task due date from any view",
			Shortcut:    ";td",
			Category:    CategoryDueDate,
			Aliases:     []string{"set due date", "change due date", "update due date"},
			Handler: func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeDueDate}
			},
		},
		{
			Name:        "global task move",
			Description: "Move a task to another project from any view",
			Shortcut:    ";tm",
			Category:    CategoryTasks,
			Aliases:     []string{"move task", "cross-project move", "transfer task"},
			Handler: func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeTaskMoveSource}
			},
		},
		{
			Name:        "global section change",
			Description: "Change task section from any view",
			Shortcut:    ";ts",
			Category:    CategoryTasks,
			Aliases:     []string{"set section", "change section", "update section"},
			Handler: func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeSectionChange}
			},
		},
		{
			Name:        "global linked task",
			Description: "Create or link a task from any view",
			Shortcut:    ";c",
			Category:    CategoryTasks,
			Aliases:     []string{"link task", "create linked task", "global link"},
			Handler: func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeLinkedTask}
			},
		},
		{
			Name:        "global template task",
			Description: "Create a task from a template in any project",
			Shortcut:    ";tN",
			Category:    CategoryTasks,
			Aliases:     []string{"global template", "template task", "new from template"},
			Handler: func() tea.Msg {
				return ShowGlobalProjectPickerMsg{Purpose: PickerPurposeTemplatePick}
			},
		},
	}
}
