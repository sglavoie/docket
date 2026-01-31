package tui

func (m *AppModel) priorityCommands() []Command {
	return []Command{
		{
			Name:        "priority critical",
			Description: "Set task priority to Critical (1)",
			Shortcut:    "!1",
			Category:    CategoryPriority,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"p1", "critical priority"},
			Handler:     m.makePriorityHandler(1),
		},
		{
			Name:        "priority high",
			Description: "Set task priority to High (2)",
			Shortcut:    "!2",
			Category:    CategoryPriority,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"p2", "high priority"},
			Handler:     m.makePriorityHandler(2),
		},
		{
			Name:        "priority normal",
			Description: "Set task priority to Normal (3)",
			Shortcut:    "!3",
			Category:    CategoryPriority,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"p3", "normal priority"},
			Handler:     m.makePriorityHandler(3),
		},
		{
			Name:        "priority low",
			Description: "Set task priority to Low (4)",
			Shortcut:    "!4",
			Category:    CategoryPriority,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"p4", "low priority"},
			Handler:     m.makePriorityHandler(4),
		},
		{
			Name:        "priority someday",
			Description: "Set task priority to Someday (5)",
			Shortcut:    "!5",
			Category:    CategoryPriority,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"p5", "someday priority"},
			Handler:     m.makePriorityHandler(5),
		},
	}
}
