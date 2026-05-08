package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// buildKeyMap constructs the declarative KeyMap for KanbanModel.
func (m *KanbanModel) buildKeyMap() *keymap.KeyMap {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	hasCard := func() bool { return m.getCursorCard() != nil }

	var entries []keymap.Entry

	// --- Column navigation (not using ListNavigationGroup — kanban is column-based) ---
	entries = append(entries,
		keymap.Bind("j", "Move cursor down within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorDown()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorUp()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("h", "Focus previous column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveFocusPrevCol()
			return m, nil
		}),
		keymap.Bind("l", "Focus next column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveFocusNextCol()
			return m, nil
		}),
		keymap.Bind("ctrl+u", "Page up within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.pageUp()
			return m, nil
		}),
		keymap.Bind("ctrl+d", "Page down within column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.pageDown()
			return m, nil
		}),
		keymap.Bind("G", "Jump to bottom of column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.jumpToBottom()
			return m, nil
		}),
		keymap.Bind("enter", "Open task detail", "Navigation", func() (tea.Model, tea.Cmd) {
			if cmd := m.openTaskDetailCmd(); cmd != nil {
				return m, cmd
			}
			return m, nil
		}),
	)

	// --- H/L: move task to adjacent column; J/K: reorder within column ---
	entries = append(entries,
		keymap.Bind("H", "Move task to left column", "Actions", func() (tea.Model, tea.Cmd) {
			return m.moveTaskLeft()
		}),
		keymap.Bind("L", "Move task to right column", "Actions", func() (tea.Model, tea.Cmd) {
			return m.moveTaskRight()
		}),
		keymap.Bind("J", "Reorder card down", "Actions", func() (tea.Model, tea.Cmd) {
			m.reorderCardDown()
			return m, nil
		}),
		keymap.Bind("K", "Reorder card up", "Actions", func() (tea.Model, tea.Cmd) {
			m.reorderCardUp()
			return m, nil
		}),
	)

	// --- Goto group: gg (top), gp (project) ---
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return KanbanSelectMsg{Task: task, Project: project} }
		},
	})...)

	// --- z prefix: zR, zM (no-op stubs in kanban) ---
	entries = append(entries, keymap.Prefix("z", "View", "View", []keymap.Entry{
		keymap.Bind("R", "Expand all sections", "View", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("M", "Collapse all sections", "View", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	}))

	// --- View mode keys ---
	entries = append(entries,
		keymap.Bind("tab", "Toggle bottom detail pane", "View", func() (tea.Model, tea.Cmd) {
			m.detailPaneVisible = !m.detailPaneVisible
			return m, nil
		}),
		keymap.Bind("`", "Open mode picker", "View", func() (tea.Model, tea.Cmd) {
			m.showModePicker = true
			return m, nil
		}),
		keymap.Bind("~", "Cycle axis mode back", "View", func() (tea.Model, tea.Cmd) {
			m.cycleAxisModeBack()
			return m, nil
		}),
		keymap.Bind("@", "Open axis mode manager", "View", func() (tea.Model, tea.Cmd) {
			m.showAxisManager = true
			m.axisCursor = 0
			return m, nil
		}),
	)

	// --- Multi-select ---
	entries = append(entries,
		keymap.Bind("V", "Enter multi-select mode", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.ToggleMode(&m.statusMsg, "Multi-select mode — space: select, H/L: bulk move, esc: exit")
			return m, nil
		}),
		keymap.Bind("space", "Toggle multi-select on card", "Multi-select", func() (tea.Model, tea.Cmd) {
			if card := m.getCursorCard(); card != nil {
				m.ToggleAndUpdateStatus(card.Task.ID, &m.statusMsg,
					"Multi-select: navigate and press space to select",
					"%d task(s) selected")
			}
			return m, nil
		}),
		keymap.Bind("esc", "Exit multi-select", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.ExitMultiSelect()
			m.statusMsg = ""
			return m, nil
		}, keymap.WithGate(func() bool { return m.multiSelectMode })),
	)

	// --- Direct action keys ---
	entries = append(entries,
		keymap.Bind("a", "Add task (quick capture)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg {
				return ShowQuickCaptureMsg{
					Project:     m.project,
					AllProjects: m.config.GetActiveProjects(),
				}
			}
		}),
		keymap.Bind("M", "Move to custom section (picker)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.mode == KanbanModeCustomSections {
				card := m.getCursorCard()
				if card != nil {
					projects := m.loadedProjects
					return m, func() tea.Msg {
						return ShowCustomSectionPickerMsg{Task: card.Task, Project: card.Project, Projects: projects}
					}
				}
			}
			return m, nil
		}),
		keymap.Bind("r", "Refresh", "Actions", func() (tea.Model, tea.Cmd) {
			m.reloadData()
			m.clampCursor()
			m.setStatus("Refreshed")
			return m, nil
		}),
	)

	// --- Status group: "," prefix, gated on hasCard ---
	statusEntries := keymap.StatusGroup(keymap.StatusDeps{
		ChangeStatus: func(status string) keymap.Action {
			s := models.TaskStatus(status)
			return func() (tea.Model, tea.Cmd) {
				return taskOpsChangeStatus(m, s)
			}
		},
		HasRecurrence:  nil,
		SkipRecurrence: nil,
		DeferChildren:  nil,
	})
	if len(statusEntries) > 0 && statusEntries[0].PrefixNode != nil {
		statusEntries[0].PrefixNode.Gate = hasCard
	}
	entries = append(entries, statusEntries...)

	// --- Priority group: "!" prefix, gated on hasCard ---
	priorityEntries := keymap.PriorityGroup(keymap.PriorityDeps{
		ChangePriority: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return taskOpsChangePriority(m, key)
			}
		},
	})
	if len(priorityEntries) > 0 && priorityEntries[0].PrefixNode != nil {
		priorityEntries[0].PrefixNode.Gate = hasCard
	}
	entries = append(entries, priorityEntries...)

	// --- Due date group: "d" prefix, gated on hasCard ---
	bulkSetDueDate := func(days int) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.SelectedCount() > 0 {
				dueDate := util.StartOfDay(time.Now().AddDate(0, 0, days))
				var count int
				for _, col := range m.columns {
					for i := range col.Tasks {
						card := &col.Tasks[i]
						if !m.IsSelected(card.Task.ID) {
							continue
						}
						pf := workflowPersist(func() WorkflowSaveOptions {
							return BuildWorkflowOpts(m.getPersister(), card.Project, m.config, m.configManager, nil, "", "", nil)
						})
						d := dueDate
						pf(card.Project, card.Task.ID, func(t *models.Task) {
							t.DueDate = &d
							t.HasTime = false
						})
						count++
					}
				}
				m.ExitMultiSelect()
				m.setStatus(fmt.Sprintf("Due date set on %d tasks", count))
				m.reloadData()
				m.clampCursor()
				return m, nil
			}
			return taskOpsSetQuickDueDate(m, days)
		}
	}
	dueDateEntries := keymap.DueDateGroup(keymap.DueDateDeps{
		DueToday:    bulkSetDueDate(0),
		DueTomorrow: bulkSetDueDate(1),
		DueInAWeek:  bulkSetDueDate(7),
		DueInAMonth: bulkSetDueDate(30),
		ClearDueDate: func() (tea.Model, tea.Cmd) {
			return taskOpsClearDueDate(m)
		},
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return SetDueDateMsg{Task: task, Project: project} }
		},
	})
	if len(dueDateEntries) > 0 && dueDateEntries[0].PrefixNode != nil {
		dueDateEntries[0].PrefixNode.Gate = hasCard
	}
	entries = append(entries, dueDateEntries...)

	// --- Snooze group: "s" prefix, gated on hasCard ---
	snoozeDateAction := func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			snoozeDate, _, ok := resolveQuickSnoozeDate(key)
			if !ok {
				return m, nil
			}
			return taskOpsQuickSnooze(m, snoozeDate)
		}
	}
	snoozeEntries := keymap.SnoozeGroup(keymap.SnoozeDeps{
		SnoozeTomorrow:   snoozeDateAction("t"),
		SnoozeOneWeek:    snoozeDateAction("w"),
		SnoozeOneMonth:   snoozeDateAction("m"),
		SnoozeNextMonday: snoozeDateAction("n"),
		ClearSnooze:      snoozeDateAction("0"),
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return SetSnoozeDateMsg{Task: task, Project: project} }
		},
	})
	if len(snoozeEntries) > 0 && snoozeEntries[0].PrefixNode != nil {
		snoozeEntries[0].PrefixNode.Gate = hasCard
	}
	entries = append(entries, snoozeEntries...)

	// --- Move group: "m" prefix, gated on hasCard ---
	moveEntries := keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: func(section string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return taskOpsSectionMove(m, section)
			}
		},
		MoveToTop:    func() (tea.Model, tea.Cmd) { return m, nil },
		MoveToBottom: func() (tea.Model, tea.Cmd) { return m, nil },
	})
	if len(moveEntries) > 0 && moveEntries[0].PrefixNode != nil {
		moveEntries[0].PrefixNode.Gate = hasCard
	}
	entries = append(entries, moveEntries...)

	// --- Linked task group: "c" prefix, gated on hasCard ---
	linkedEntries := keymap.LinkedTaskGroup(keymap.LinkedTaskDeps{
		CreateBlocker: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeBlocker}
			}
		},
		CreateDependent: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeDependent}
			}
		},
		CreateSubtask: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeSubtask}
			}
		},
		CreateFollowUp: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeFollowUp}
			}
		},
		NavigateLinked: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowLinkedTaskSelectorMsg{Task: task, Project: project} }
		},
		CrossProjectLink: nil,
		UnlinkTask:       nil,
	})
	if len(linkedEntries) > 0 && linkedEntries[0].PrefixNode != nil {
		linkedEntries[0].PrefixNode.Gate = hasCard
	}
	entries = append(entries, linkedEntries...)

	// --- Task prefix group: "t" prefix, gated on hasCard ---
	taskPrefixEntries := keymap.TaskPrefixGroup(keymap.TaskPrefixDeps{
		NewTask: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return CreateTaskMsg{ParentTask: task, SkipTemplatePicker: true} }
		},
		NewTaskWithTemplate: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return CreateTaskMsg{ParentTask: task, SkipTemplatePicker: false} }
		},
		RenameTask: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return EditTaskMsg{Task: task, Project: project} }
		},
		ChangeSection: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowSectionPickerMsg{Task: task, Project: project} }
		},
		OpenEditor: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return EditTaskContentMsg{Task: task, Project: project} }
		},
		TogglePin: func() (tea.Model, tea.Cmd) { return taskOpsTogglePinned(m) },
		ToggleDim: func() (tea.Model, tea.Cmd) { return taskOpsToggleDimmed(m) },
		ManageTags: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			tags := append([]string{}, task.Tags...)
			return m, func() tea.Msg {
				return ShowTagSelectorMsg{Task: task, Project: project, CurrentTags: tags}
			}
		},
		ViewComments: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowTaskCommentsMsg{Task: task, Project: project} }
		},
		ViewAttachments: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowTaskAttachmentsMsg{Task: task, Project: project} }
		},
		EditChecklist: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowTaskChecklistMsg{Task: task, Project: project} }
		},
		EditDescription: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowDescriptionEditorMsg{Task: task, Project: project} }
		},
		MoveToProject: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
		},
		ViewDecisions: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowTaskDecisionsMsg{Task: task, Project: project} }
		},
		DeleteTask: func() (tea.Model, tea.Cmd) {
			return taskOpsDeleteTask(m)
		},
	})
	if len(taskPrefixEntries) > 0 && taskPrefixEntries[0].PrefixNode != nil {
		taskPrefixEntries[0].PrefixNode.Gate = hasCard
	}
	entries = append(entries, taskPrefixEntries...)

	// --- Stage group: "." prefix, gated on hasCard ---
	stageEntries := keymap.StageGroup(keymap.StageDeps{
		Stages: stages,
		ChangeStage: func(stage string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				return taskOpsChangeStage(m, stage)
			}
		},
	})
	if len(stageEntries) > 0 && stageEntries[0].PrefixNode != nil {
		stageEntries[0].PrefixNode.Gate = hasCard
	}
	entries = append(entries, stageEntries...)

	// --- Blocker group: "b" prefix, gated on hasCard ---
	blockerEntries := keymap.BlockerGroup(keymap.BlockerDeps{
		ViewDependencies: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowDependencyViewerMsg{Task: task, Project: project} }
		},
		ManageDependencies: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ManageDependenciesMsg{Task: task, Project: project} }
		},
	})
	if len(blockerEntries) > 0 && blockerEntries[0].PrefixNode != nil {
		blockerEntries[0].PrefixNode.Gate = hasCard
	}
	entries = append(entries, blockerEntries...)

	// --- Filter group: "F" prefix, no gate ---
	filterEntries := keymap.FilterGroup(keymap.FilterDeps{
		FilterByTags:     func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("#") },
		TogglePriority:   func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("p") },
		ToggleHideDimmed: func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("h") },
		ClearFilters:     func() (tea.Model, tea.Cmd) { return m.handleFilterPrefix("x") },
		TypeKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Ft", key) }
		},
		StatusKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fs", key) }
		},
		DueDateKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fd", key) }
		},
		AttrDirectKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("Fa", key) }
		},
		AttrSubKey: func(prefix, key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix(prefix, key) }
		},
		ScoreKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("FS", key) }
		},
		StageFilterKey: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) { return m.handleFilterSubPrefix("F.", key) }
		},
		StageFilterNames: stages,
	})
	entries = append(entries, filterEntries...)

	// --- Common task actions: o, Z, D, e, ?, *, S, O, ctrl+b, ctrl+. ---
	entries = append(entries, keymap.CommonTaskActionGroup(keymap.CommonTaskActionDeps{
		ViewTask: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowTaskDetailMsg{Task: task, Project: project} }
		},
		SnoozeDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return SetSnoozeDateMsg{Task: task, Project: project} }
		},
		MarkDone: func() (tea.Model, tea.Cmd) {
			return taskOpsChangeStatus(m, models.StatusDone)
		},
		EditTask: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return EditTaskMsg{Task: task, Project: project} }
		},
		ToggleHelp: func() (tea.Model, tea.Cmd) {
			return m, helpCmd(m.viewID(), m.viewName(), m.GetHelpItems())
		},
		ToggleActive: func() (tea.Model, tea.Cmd) {
			return taskOpsToggleActive(m)
		},
		EditScore: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowScoreEditorMsg{Task: task, Project: project} }
		},
		OpenURL: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task == nil || task.URL == "" {
				return m, nil
			}
			return m, openBrowserCmd(task.URL)
		},
		MoveToProject: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
		},
		QuickNote: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return OpenQuickNoteMsg{Task: task, Project: project} }
		},
	})...)

	viewName := "Kanban"
	if m.crossProject {
		viewName = "Cross-Project Kanban"
	}
	return &keymap.KeyMap{
		ViewName: viewName,
		Entries:  entries,
	}
}

// HandledKeys implements KeySyncView.
func (m *KanbanModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}
