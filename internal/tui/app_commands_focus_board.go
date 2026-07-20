package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) focusBoardCommands() []Command {
	focusBoardViews := []ViewType{ViewFocusBoard}
	return []Command{
		{
			Name:        "today task details",
			Description: "Show details for the selected Today task",
			Shortcut:    "enter",
			Category:    CategoryNavigation,
			Views:       focusBoardViews,
			Aliases:     []string{"today details", "view today task details", "inspect today task"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandOpenTask)
			},
		},
		{
			Name:        "today complete or dismiss",
			Description: "Complete the selected Today task or dismiss blocked work",
			Shortcut:    "x",
			Category:    CategoryActions,
			Views:       focusBoardViews,
			Aliases:     []string{"today done", "complete today task", "dismiss today task"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandCompleteTask)
			},
		},
		{
			Name:        "today set priority",
			Description: "Set priority for the selected Today task",
			Shortcut:    "p",
			Category:    CategoryPriority,
			Views:       focusBoardViews,
			Aliases:     []string{"today priority", "prioritize today task", "set today priority"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandPrioritizeTask)
			},
		},
		{
			Name:        "today plan backlog",
			Description: "Open Planner — Backlog to plan more work for Today",
			Shortcut:    "p",
			Category:    CategoryActions,
			Views:       focusBoardViews,
			Aliases:     []string{"plan today", "plan more", "choose today work", "today backlog"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandPlanToday)
			},
		},
		{
			Name:        "today triage inbox",
			Description: "Open global inbox triage from Today",
			Shortcut:    "ctrl+i",
			Category:    CategoryActions,
			Views:       focusBoardViews,
			Aliases:     []string{"triage today inbox", "global inbox", "process inbox"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandTriageInbox)
			},
		},
		{
			Name:        "today quick add",
			Description: "Add a task directly from Today",
			Shortcut:    "a",
			Category:    CategoryTasks,
			Views:       focusBoardViews,
			Aliases:     []string{"today add", "add today task", "quick add today", "quick capture"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandQuickCapture)
			},
		},
		{
			Name:        "today search",
			Description: "Search and filter Today",
			Shortcut:    "/",
			Category:    CategorySearch,
			Views:       focusBoardViews,
			Aliases:     []string{"filter today", "find today task"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandSearchFilter)
			},
		},
		{
			Name:        "today unblock task",
			Description: "Unblock the selected Today task when blockers are resolved",
			Shortcut:    "u",
			Category:    CategoryStatus,
			Views:       focusBoardViews,
			Aliases:     []string{"unblock today", "reopen blocked today task"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandUnblockTask)
			},
		},
		{
			Name:        "today next tab",
			Description: "Switch to the next Today tab",
			Shortcut:    "tab",
			Category:    CategoryNavigation,
			Views:       focusBoardViews,
			Aliases:     []string{"switch today tab", "today tabs"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandSwitchTab)
			},
		},
		{
			Name:        "today new project",
			Description: "Create a project from the first-run Today state",
			Shortcut:    "p",
			Category:    CategoryProjects,
			Views:       focusBoardViews,
			Aliases:     []string{"new project from today", "create project"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandNewProject)
			},
		},
		{
			Name:        "today pipeline hub",
			Description: "Open the Pipeline Hub from Today",
			Shortcut:    ";;",
			Category:    CategoryViews,
			Views:       focusBoardViews,
			Aliases:     []string{"pipeline hub", "hub", "pipeline"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandOpenPipelineHub)
			},
		},
		{
			Name:        "today help",
			Description: "Show Today help",
			Shortcut:    "?",
			Category:    CategoryHelp,
			Views:       focusBoardViews,
			Aliases:     []string{"today shortcuts", "today keys"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandShowHelp)
			},
		},
		{
			Name:        "today close",
			Description: "Return from Today to the previous view",
			Shortcut:    "esc",
			Category:    CategoryNavigation,
			Views:       focusBoardViews,
			Aliases:     []string{"close today", "leave today"},
			Handler: func() tea.Msg {
				return m.runFocusBoardCockpitCommand(CockpitCommandCloseView)
			},
		},
	}
}

func (m *AppModel) runFocusBoardCockpitCommand(command CockpitCommand) tea.Msg {
	focusBoard, ok := m.views.Get(ViewFocusBoard).(*FocusBoardModel)
	if !ok || focusBoard == nil {
		return nil
	}
	action := focusBoard.cockpitActionForPaletteCommand(command)
	if action.IsZero() {
		return nil
	}
	_, cmd := focusBoard.executeCockpitAction(action)
	if cmd == nil {
		return nil
	}
	return PaletteCommandResultMsg{Cmd: cmd}
}

func (m *FocusBoardModel) cockpitActionForPaletteCommand(command CockpitCommand) CockpitAction {
	switch command {
	case CockpitCommandOpenTask:
		return actionOpenTask()
	case CockpitCommandCompleteTask, CockpitCommandDismissTask:
		return m.resolveCockpitActionForKey("x")
	case CockpitCommandPrioritizeTask:
		if row := m.getRowAtCursor(); row != nil && row.isTask() {
			return actionPrioritizeTask()
		}
		return CockpitAction{}
	case CockpitCommandPlanToday:
		return actionPlanToday()
	case CockpitCommandPlanMore:
		return actionPlanMore()
	case CockpitCommandTriageInbox:
		return actionTriageInbox()
	case CockpitCommandUnblockTask:
		return actionUnblockTask()
	case CockpitCommandQuickCapture:
		return actionQuickCapture()
	case CockpitCommandSearchFilter:
		return actionSearchFilter()
	case CockpitCommandNewProject:
		return actionNewProject()
	case CockpitCommandSwitchTab:
		return actionSwitchTab()
	case CockpitCommandCloseView:
		return actionCloseView()
	case CockpitCommandOpenPipelineHub:
		return actionOpenPipelineHub()
	case CockpitCommandOpenCommandPalette:
		return actionOpenCommandPalette()
	case CockpitCommandShowHelp:
		return actionShowHelp()
	default:
		return CockpitAction{}
	}
}

func (m *AppModel) focusBoardSuggestedCommandOrder() []string {
	var order []string
	seen := make(map[string]bool)
	appendName := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		order = append(order, name)
	}
	appendCommand := func(command CockpitCommand) {
		if name, ok := focusBoardCommandName(command); ok {
			appendName(name)
		}
	}
	appendAction := func(action CockpitAction) {
		appendCommand(action.Command)
	}

	if focusBoard, ok := m.views.Get(ViewFocusBoard).(*FocusBoardModel); ok && focusBoard != nil {
		rec := focusBoard.resolveCockpitRecommendation()
		appendAction(rec.PrimaryAction)
		appendAction(rec.SecondaryAction)
		for _, action := range rec.Footer.Contextual {
			appendAction(action)
		}
		appendCommand(CockpitCommandSearchFilter)
		appendCommand(CockpitCommandOpenPipelineHub)
		for _, action := range rec.Footer.Escape {
			appendAction(action)
		}
	}

	for _, name := range defaultFocusBoardSuggestedCommandOrder {
		appendName(name)
	}
	return order
}

var defaultFocusBoardSuggestedCommandOrder = []string{
	"today task details",
	"today complete or dismiss",
	"today set priority",
	"today quick add",
	"today search",
}

func focusBoardCommandName(command CockpitCommand) (string, bool) {
	switch command {
	case CockpitCommandOpenTask:
		return "today task details", true
	case CockpitCommandCompleteTask, CockpitCommandDismissTask:
		return "today complete or dismiss", true
	case CockpitCommandPrioritizeTask:
		return "today set priority", true
	case CockpitCommandPlanToday, CockpitCommandPlanMore:
		return "today plan backlog", true
	case CockpitCommandTriageInbox:
		return "today triage inbox", true
	case CockpitCommandUnblockTask:
		return "today unblock task", true
	case CockpitCommandQuickCapture:
		return "today quick add", true
	case CockpitCommandSearchFilter:
		return "today search", true
	case CockpitCommandNewProject:
		return "today new project", true
	case CockpitCommandSwitchTab:
		return "today next tab", true
	case CockpitCommandCloseView:
		return "today close", true
	case CockpitCommandOpenPipelineHub:
		return "today pipeline hub", true
	case CockpitCommandShowHelp:
		return "today help", true
	default:
		return "", false
	}
}
