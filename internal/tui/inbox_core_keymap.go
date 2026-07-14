package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for inboxCore.
// Both InboxProcessorModel and GlobalInboxProcessorModel embed inboxCore and
// call this via initCore to wire the Dispatcher.
//
//nolint:gocyclo // This is a declarative inventory of the complete inbox key vocabulary.
func (c *inboxCore) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// --- List navigation: j/k, G ---
	entries = append(entries,
		keymap.Bind("j", "Next task", "Navigation", func() (tea.Model, tea.Cmd) {
			return c.model, c.advanceToNext(false)
		}, keymap.AsHidden()),
		keymap.Bind("down", "Next task", "Navigation", func() (tea.Model, tea.Cmd) {
			return c.model, c.advanceToNext(false)
		}, keymap.AsHidden()),
		keymap.Bind("k", "Previous task", "Navigation", func() (tea.Model, tea.Cmd) {
			c.goToPrevious()
			return c.model, nil
		}, keymap.AsHidden()),
		keymap.Bind("up", "Previous task", "Navigation", func() (tea.Model, tea.Cmd) {
			c.goToPrevious()
			return c.model, nil
		}, keymap.AsHidden()),
		keymap.Bind("G", "Jump to last task", "Navigation", func() (tea.Model, tea.Cmd) {
			if c.backend.taskCount() > 0 {
				c.currentIndex = c.backend.taskCount() - 1
			}
			return c.model, nil
		}),
	)

	// --- Goto group: g prefix (gg = top) ---
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) {
			c.currentIndex = 0
			return c.model, nil
		},
		GoToProject: nil,
	})...)

	// --- Detail view (enter) — kept separate from the primary triage vocabulary
	// because enter is a navigation aid, not a triage action. ---
	entries = append(entries,
		keymap.Bind("enter", "View task details", "Navigation", func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task != nil {
				return c.model, func() tea.Msg {
					return ShowTaskDetailMsg{Task: task, Project: project}
				}
			}
			return c.model, nil
		}),
	)

	// --- Direct priority keys: 1-5 (quick set; PriorityGroup below is the
	// prefix-vocabulary equivalent). ---
	for _, k := range []string{"1", "2", "3", "4", "5"} {
		key := k
		entries = append(entries,
			keymap.Bind(key, "Set priority "+key+" (quick)", "Operations", func() (tea.Model, tea.Cmd) {
				return c.model, c.setPriorityQuick(key)
			}),
		)
	}

	// --- Primary triage actions: n, e, D, T, u, ? ---
	entries = append(entries, keymap.TriagePrimaryGroup(keymap.TriagePrimaryDeps{
		Skip: func() (tea.Model, tea.Cmd) {
			return c.model, c.skipCurrent()
		},
		Edit: func() (tea.Model, tea.Cmd) {
			return c.model, inboxOpenTaskEditor(c)
		},
		MarkDone: func() (tea.Model, tea.Cmd) {
			return c.model, c.changeTaskState(models.StateDone, StateVerb(models.StateDone))
		},
		ManageTags: func() (tea.Model, tea.Cmd) {
			return c.model, inboxOpenTagSelector(c)
		},
		Undo: func() (tea.Model, tea.Cmd) {
			return c.model, c.undoLastAction()
		},
		ToggleHelp: func() (tea.Model, tea.Cmd) {
			return c.model, helpCmd(c.backend.viewType(), c.backend.helpViewName(), c.GetHelpItems())
		},
	})...)

	// --- Secondary inbox-specific shortcuts (not part of the shared primary
	// vocabulary; kept as direct keys because the local picker affordances are
	// useful when the user does not remember the section name). ---
	entries = append(entries,
		keymap.Bind("m", "Move from Inbox to project", "Operations", func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil || project == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg {
				return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project}
			}
		}, keymap.WithGate(func() bool {
			if c.backend == nil {
				return true
			}
			_, project := c.currentTaskRef()
			return project != nil && c.runtime.IsSystemInboxProject(project.ID)
		})),
		keymap.Bind("p", "Create project for Inbox item", "Operations", func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil || project == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg {
				return CreateProjectMsg{MoveTask: task, SourceProject: project}
			}
		}, keymap.WithGate(func() bool {
			if c.backend == nil {
				return true
			}
			_, project := c.currentTaskRef()
			return project != nil && c.runtime.IsSystemInboxProject(project.ID)
		})),
		keymap.Bind("ctrl+w", "Send to Waiting section", "Operations", func() (tea.Model, tea.Cmd) {
			return c.model, c.sendToWaiting()
		}),
		keymap.Bind("ctrl+s", "Assign section (picker)", "Operations", func() (tea.Model, tea.Cmd) {
			return c.model, inboxOpenSectionPicker(c)
		}),
	)

	// --- Status group: "," prefix ---
	statusEntries := keymap.StatusGroup(keymap.StatusDeps{
		ChangeState: func(state string) keymap.Action {
			s := models.WorkflowState(state)
			return func() (tea.Model, tea.Cmd) {
				return c.model, c.changeTaskState(s, StateVerb(s))
			}
		},
		HasRecurrence:  nil,
		SkipRecurrence: nil,
		DeferChildren:  nil,
	})
	entries = append(entries, statusEntries...)

	// --- Priority group: "!" prefix ---
	priorityEntries := keymap.PriorityGroup(keymap.PriorityDeps{
		ChangePriority: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				priority, _ := PriorityForPrefixKey(key)
				return c.model, c.changePriority(priority)
			}
		},
	})
	entries = append(entries, priorityEntries...)

	// --- Snooze group: "z" prefix ---
	snoozeEntries := keymap.SnoozeGroup(keymap.SnoozeDeps{
		SnoozeTomorrow: func() (tea.Model, tea.Cmd) {
			snoozeDate, _, ok := resolveQuickSnoozeDate("t")
			if !ok {
				return c.model, nil
			}
			return c.model, c.setSnooze(snoozeDate)
		},
		SnoozeOneWeek: func() (tea.Model, tea.Cmd) {
			snoozeDate, _, ok := resolveQuickSnoozeDate("w")
			if !ok {
				return c.model, nil
			}
			return c.model, c.setSnooze(snoozeDate)
		},
		SnoozeOneMonth: func() (tea.Model, tea.Cmd) {
			snoozeDate, _, ok := resolveQuickSnoozeDate("m")
			if !ok {
				return c.model, nil
			}
			return c.model, c.setSnooze(snoozeDate)
		},
		SnoozeNextMonday: func() (tea.Model, tea.Cmd) {
			snoozeDate, _, ok := resolveQuickSnoozeDate("n")
			if !ok {
				return c.model, nil
			}
			return c.model, c.setSnooze(snoozeDate)
		},
		ClearSnooze: func() (tea.Model, tea.Cmd) {
			snoozeDate, _, ok := resolveQuickSnoozeDate("0")
			if !ok {
				return c.model, nil
			}
			return c.model, c.setSnooze(snoozeDate)
		},
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			return c.model, c.openSnoozePicker()
		},
	})
	entries = append(entries, snoozeEntries...)

	// --- Move group: "m" prefix ---
	moveEntries := keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: func(section string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return c.model, c.moveToSection(section)
			}
		},
		MoveToTop:    func() (tea.Model, tea.Cmd) { return c.model, nil },
		MoveToBottom: func() (tea.Model, tea.Cmd) { return c.model, nil },
	})
	entries = append(entries, moveEntries...)

	// --- Due date group: "d" prefix ---
	dueDateEntries := keymap.DueDateGroup(keymap.DueDateDeps{
		DueToday:     func() (tea.Model, tea.Cmd) { return c.model, c.setQuickDueDate(0) },
		DueTomorrow:  func() (tea.Model, tea.Cmd) { return c.model, c.setQuickDueDate(1) },
		DueInAWeek:   func() (tea.Model, tea.Cmd) { return c.model, c.setQuickDueDate(7) },
		DueInAMonth:  func() (tea.Model, tea.Cmd) { return c.model, c.setQuickDueDate(30) },
		ClearDueDate: func() (tea.Model, tea.Cmd) { return c.model, c.clearDueDate() },
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			return c.model, inboxOpenDueDateInput(c)
		},
	})
	entries = append(entries, dueDateEntries...)

	// --- Linked task group: "c" prefix ---
	linkedEntries := keymap.LinkedTaskGroup(keymap.LinkedTaskDeps{
		CreateBlocker: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeBlocker}
			}
		},
		CreateDependent: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeDependent}
			}
		},
		CreateSubtask: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeSubtask}
			}
		},
		CreateFollowUp: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeFollowUp}
			}
		},
		NavigateLinked: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return ShowLinkedTaskSelectorMsg{Task: task, Project: project} }
		},
		CrossProjectLink: nil,
		UnlinkTask:       nil,
	})
	entries = append(entries, linkedEntries...)

	// --- Task prefix group: "t" prefix ---
	taskPrefixEntries := keymap.TaskPrefixGroup(keymap.TaskPrefixDeps{
		NewTask: func() (tea.Model, tea.Cmd) {
			task, _ := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return CreateTaskMsg{ParentTask: task, SkipTemplatePicker: true} }
		},
		NewTaskWithTemplate: func() (tea.Model, tea.Cmd) {
			return c.model, inboxOpenSubtaskCreator(c)
		},
		RenameTask: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return EditTaskMsg{Task: task, Project: project} }
		},
		ChangeSection: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return ShowSectionPickerMsg{Task: task, Project: project} }
		},
		OpenEditor: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return EditTaskContentMsg{Task: task, Project: project} }
		},
		TogglePin: func() (tea.Model, tea.Cmd) { return c.model, nil },
		ToggleDim: func() (tea.Model, tea.Cmd) { return c.model, nil },
		ManageTags: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			tags := append([]string{}, task.Tags...)
			return c.model, func() tea.Msg {
				return ShowTagSelectorMsg{Task: task, Project: project, CurrentTags: tags}
			}
		},
		ViewComments: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return ShowTaskCommentsMsg{Task: task, Project: project} }
		},
		ViewAttachments: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return ShowTaskAttachmentsMsg{Task: task, Project: project} }
		},
		EditChecklist: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return ShowTaskChecklistMsg{Task: task, Project: project} }
		},
		EditDescription: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return ShowDescriptionEditorMsg{Task: task, Project: project} }
		},
		MoveToProject: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
		},
		ViewDecisions: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return ShowTaskDecisionsMsg{Task: task, Project: project} }
		},
		ParkTask: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return ShowParkInputMsg{Task: task, Project: project} }
		},
		DeleteTask: func() (tea.Model, tea.Cmd) {
			inboxDeletePrompt(c, &c.confirm)
			return c.model, nil
		},
	})
	entries = append(entries, taskPrefixEntries...)

	// --- Blocker group: "b" prefix ---
	blockerEntries := keymap.BlockerGroup(keymap.BlockerDeps{
		ViewDependencies: func() (tea.Model, tea.Cmd) {
			task, project := c.currentTaskRef()
			if task == nil {
				return c.model, nil
			}
			return c.model, func() tea.Msg { return ShowDependencyViewerMsg{Task: task, Project: project} }
		},
		ManageDependencies: func() (tea.Model, tea.Cmd) {
			return c.model, inboxOpenDependencySelector(c)
		},
	})
	entries = append(entries, blockerEntries...)

	viewName := "Inbox Processor"
	if c.backend != nil {
		viewName = c.backend.helpViewName()
	}
	return &keymap.KeyMap{
		ViewName: viewName,
		Entries:  entries,
	}
}
