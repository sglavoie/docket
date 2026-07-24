package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) activeInboxCore() (*inboxCore, *ConfirmDialog) {
	switch m.nonPaletteOrigin() {
	case ViewInboxProcessor:
		if m.inboxProcessor != nil {
			core := &m.inboxProcessor.inboxCore
			return core, &core.confirm
		}
	case ViewGlobalInboxProcessor:
		if m.globalInboxProcessor != nil {
			core := &m.globalInboxProcessor.inboxCore
			return core, &core.confirm
		}
	default:
	}
	return nil, nil
}

func (m *AppModel) inboxProcessorCommands() []Command {
	bothViews := []PresentationID{ViewInboxProcessor, ViewGlobalInboxProcessor}

	// runInboxCmd invokes a shared inbox helper that returns a tea.Cmd, runs it
	// synchronously, and returns the produced Msg. Use this for handlers whose
	// inner Msg should be delivered directly to the palette executor (modal
	// keep-open types like EditTaskMsg / ShowSectionPickerMsg, or one-shot
	// signals like ShowTagSelectorMsg / ShowTaskDetailMsg).
	runInboxCmd := func(build func(inboxOpsView) tea.Cmd) tea.Msg {
		c, _ := m.activeInboxCore()
		if c == nil {
			return nil
		}
		cmd := build(c)
		if cmd == nil {
			return nil
		}
		return cmd()
	}

	// runInboxResultCmd wraps the tea.Cmd returned by a direct inboxCore
	// mutation in PaletteCommandResultMsg so the palette executor returns the
	// cmd to the runtime after closing the palette. Handlers that need to
	// persist task state (save) or chain advanceToNext use this so the async
	// cmd survives the popView round-trip instead of being collapsed into a
	// synchronous Msg.
	runInboxResultCmd := func(build func(*inboxCore) tea.Cmd) tea.Msg {
		c, _ := m.activeInboxCore()
		if c == nil {
			return nil
		}
		cmd := build(c)
		if cmd == nil {
			return nil
		}
		return PaletteCommandResultMsg{Cmd: cmd}
	}

	return []Command{
		// --- Primary triage vocabulary (mirrors TriagePrimaryGroup: n e D T u) ---
		{
			Name:        "skip task",
			Description: "Skip this task and advance to the next inbox item",
			Shortcut:    "n",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"next inbox task", "pass", "triage skip", "next task"},
			Handler: func() tea.Msg {
				return runInboxResultCmd(func(c *inboxCore) tea.Cmd {
					return c.skipCurrent()
				})
			},
		},
		{
			Name:        "mark task done",
			Description: "Resolve the current task and advance",
			Shortcut:    "D",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"complete task", "finish task", "resolve task", "triage done"},
			Handler: func() tea.Msg {
				return runInboxResultCmd(func(c *inboxCore) tea.Cmd {
					return c.changeTaskState(models.StateDone, StateVerb(models.StateDone))
				})
			},
		},
		{
			Name:        "edit inbox task",
			Description: "Refine title, description, or metadata of the current task",
			Shortcut:    "e",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"edit task", "refine task", "triage edit"},
			Handler: func() tea.Msg {
				return runInboxCmd(inboxOpenTaskEditor)
			},
		},
		{
			Name:        "manage inbox task tags",
			Description: "Add or remove tags on the current task",
			Shortcut:    "T",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"tag inbox task", "manage tags", "triage tags", "refine tags"},
			Handler: func() tea.Msg {
				return runInboxCmd(inboxOpenTagSelector)
			},
		},
		{
			Name:        "undo last triage action",
			Description: "Revert the most recent triage decision",
			Shortcut:    "u",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"undo", "inbox undo", "revert inbox", "undo triage"},
			Handler: func() tea.Msg {
				return runInboxResultCmd(func(c *inboxCore) tea.Cmd {
					return c.undoLastAction()
				})
			},
		},
		// --- Navigation / details ---
		{
			Name:        "view task details",
			Description: "Open the detail view for the current task",
			Shortcut:    "enter",
			Category:    CategoryNavigation,
			Views:       bothViews,
			Aliases:     []string{"task details", "open task"},
			Handler: func() tea.Msg {
				c, _ := m.activeInboxCore()
				if c == nil {
					return nil
				}
				task, project := c.currentTaskRef()
				if task == nil {
					return nil
				}
				return ShowTaskDetailMsg{Task: task, Project: project}
			},
		},
		// --- Route & schedule (section, waiting, due date) ---
		{
			Name:        "route to section",
			Description: "Open the section picker for the current task",
			Shortcut:    "ctrl+s",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"assign section", "pick section", "move to section", "route inbox"},
			Handler: func() tea.Msg {
				return runInboxCmd(inboxOpenSectionPicker)
			},
		},
		{
			Name:        "send to next",
			Description: "Route the current task to the Next (on-deck) section",
			Shortcut:    "mn",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"schedule next", "move to next", "on deck", "triage next"},
			Handler: func() tea.Msg {
				return runInboxResultCmd(func(c *inboxCore) tea.Cmd {
					return c.moveToSection(models.SectionNext)
				})
			},
		},
		{
			Name:        "plan today",
			Description: "Toggle the current task in today's daily plan",
			Shortcut:    "w",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"toggle daily plan", "add to today", "today plan"},
			Handler: func() tea.Msg {
				return TogglePlanTodayFromPaletteMsg{}
			},
		},
		{
			Name:        "send to waiting",
			Description: "Move the current task to the Waiting section",
			Shortcut:    "ctrl+w",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"waiting", "defer task", "triage wait"},
			Handler: func() tea.Msg {
				return runInboxResultCmd(func(c *inboxCore) tea.Cmd {
					return c.sendToWaiting()
				})
			},
		},
		{
			Name:        "set inbox task due date",
			Description: "Open the date picker to schedule the current task",
			Shortcut:    "dd",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"inbox due date", "due date", "deadline", "schedule task"},
			Handler: func() tea.Msg {
				return runInboxCmd(inboxOpenDueDateInput)
			},
		},
		{
			Name:        "set due date today",
			Description: "Mark the current task as due today",
			Shortcut:    "dt",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"due today", "deadline today", "today deadline"},
			Handler: func() tea.Msg {
				return runInboxResultCmd(func(c *inboxCore) tea.Cmd {
					return c.setQuickDueDate(0)
				})
			},
		},
		{
			Name:        "snooze inbox task",
			Description: "Snooze the current task until a future date",
			Shortcut:    "zd",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"snooze task", "defer until", "wake up date"},
			Handler: func() tea.Msg {
				return runInboxCmd(inboxOpenSnoozeInput)
			},
		},
		// --- Refine (subtask, dependencies) ---
		{
			Name:        "create subtask from inbox",
			Description: "Create a subtask under the current task",
			Shortcut:    "cs",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"add subtask", "subtask from inbox", "refine subtask"},
			Handler: func() tea.Msg {
				return runInboxCmd(inboxOpenSubtaskCreator)
			},
		},
		{
			Name:        "manage inbox dependencies",
			Description: "Manage blockers and dependents for the current task",
			Shortcut:    "bm",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"inbox dependencies", "manage blockers", "add dependency"},
			Handler: func() tea.Msg {
				return runInboxCmd(inboxOpenDependencySelector)
			},
		},
		// --- Destructive ---
		{
			Name:        "delete inbox task",
			Description: "Delete the current inbox task (asks for confirmation)",
			Shortcut:    "X",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"remove inbox task", "trash inbox task"},
			Handler: func() tea.Msg {
				c, _ := m.activeInboxCore()
				if c == nil {
					return nil
				}
				cmd := c.requestDelete()
				if cmd == nil {
					return nil
				}
				return cmd()
			},
		},
	}
}
