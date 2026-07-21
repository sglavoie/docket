package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) focusViewCommands() []Command {
	return []Command{
		{
			Name:        "go to task in project",
			Description: "Navigate to task in outliner",
			Shortcut:    "gp",
			Category:    CategoryNavigation,
			Views:       []PresentationID{ViewProjectFocusView},
			Aliases:     []string{"open in project", "show in outliner"},
			Handler: func() tea.Msg {
				return syntheticKey('g')
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewProjectFocusView},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return syntheticKey('y')
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewProjectFocusView},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return syntheticKey('Y')
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewProjectFocusView},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return syntheticCtrlKey('y')
			},
		},
	}
}

func (m *AppModel) dailyReviewCommands() []Command {
	return []Command{
		{
			Name:        "focus on due today",
			Description: "Focus on due today section",
			Shortcut:    "sd",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"due today section"},
			Handler: func() tea.Msg {
				return syntheticKey('s')
			},
		},
		{
			Name:        "focus on top priority",
			Description: "Focus on top priority section",
			Shortcut:    "st",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"top priority section"},
			Handler: func() tea.Msg {
				return syntheticKey('s')
			},
		},
		{
			Name:        "focus on stale tasks",
			Description: "Focus on stale tasks section",
			Shortcut:    "ss",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"stale section"},
			Handler: func() tea.Msg {
				return syntheticKey('s')
			},
		},
		{
			Name:        "focus on recently logged",
			Description: "Focus on recently logged section",
			Shortcut:    "sl",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"recently logged section"},
			Handler: func() tea.Msg {
				return syntheticKey('s')
			},
		},
		{
			Name:        "focus on flagged tasks",
			Description: "Focus on flagged/active tasks section",
			Shortcut:    "sf",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"flagged section", "active section"},
			Handler: func() tea.Msg {
				return syntheticKey('s')
			},
		},
		{
			Name:        "focus on overdue tasks",
			Description: "Focus on overdue tasks section",
			Shortcut:    "so",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"overdue section"},
			Handler: func() tea.Msg {
				return syntheticKey('s')
			},
		},
		{
			Name:        "collapse/expand section",
			Description: "Toggle section collapse/expand",
			Shortcut:    "tab",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"fold section", "toggle section"},
			Handler: func() tea.Msg {
				return syntheticSpecialKey(tea.KeyTab)
			},
		},
		{
			Name:        "next section",
			Description: "Jump to next section",
			Shortcut:    "n",
			Category:    CategoryNavigation,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"section forward"},
			Handler: func() tea.Msg {
				return syntheticKey('n')
			},
		},
		{
			Name:        "previous section",
			Description: "Jump to previous section",
			Shortcut:    "N",
			Category:    CategoryNavigation,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"section backward"},
			Handler: func() tea.Msg {
				return syntheticKey('N')
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return syntheticKey('y')
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return syntheticKey('Y')
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewDailyReview},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return syntheticCtrlKey('y')
			},
		},
	}
}

func (m *AppModel) priorityViewCommands() []Command {
	return []Command{
		{
			Name:        "start suggested task",
			Description: "Start the suggested task",
			Shortcut:    "s",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewPriorityView},
			Aliases:     []string{"begin suggestion", "accept suggestion"},
			Handler: func() tea.Msg {
				return syntheticKey('s')
			},
		},
		{
			Name:        "dismiss suggestion",
			Description: "Dismiss the current suggestion",
			Shortcut:    "x",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewPriorityView},
			Aliases:     []string{"skip suggestion", "reject suggestion"},
			Handler: func() tea.Msg {
				return syntheticKey('x')
			},
		},
		{
			Name:        "go to task in outliner",
			Description: "Navigate to task in outliner",
			Shortcut:    "gt",
			Category:    CategoryNavigation,
			Views:       []PresentationID{ViewPriorityView},
			Aliases:     []string{"open in outliner", "show in outliner"},
			Handler: func() tea.Msg {
				return syntheticKey('g')
			},
		},
		{
			Name:        "toggle waiting section",
			Description: "Toggle waiting-on section visibility",
			Shortcut:    "tab",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewPriorityView},
			Aliases:     []string{"show waiting", "hide waiting"},
			Handler: func() tea.Msg {
				return syntheticSpecialKey(tea.KeyTab)
			},
		},
		{
			Name:        "refresh priority view",
			Description: "Refresh priority data",
			Shortcut:    "r",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewPriorityView},
			Aliases:     []string{"reload priorities"},
			Handler: func() tea.Msg {
				return syntheticKey('r')
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewPriorityView},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return syntheticKey('y')
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewPriorityView},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return syntheticKey('Y')
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []PresentationID{ViewPriorityView},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return syntheticCtrlKey('y')
			},
		},
	}
}

// activeInboxCore returns the inboxCore (plus its confirm dialog) for the
// inbox processor that opened the palette, regardless of whether it is the
// per-project (ViewInboxProcessor) or cross-project (ViewGlobalInboxProcessor)
// variant. Palette commands scoped to both views use this to dispatch shared
// triage helpers and direct core mutations without synthetic key replay.
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
			Shortcut:    "tD",
			Category:    CategoryActions,
			Views:       bothViews,
			Aliases:     []string{"remove inbox task", "trash inbox task"},
			Handler: func() tea.Msg {
				c, confirm := m.activeInboxCore()
				if c == nil || confirm == nil {
					return nil
				}
				inboxDeletePrompt(c, confirm)
				return nil
			},
		},
	}
}
