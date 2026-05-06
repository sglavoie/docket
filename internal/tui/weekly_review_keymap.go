package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// buildKeyMap constructs the declarative KeyMap for WeeklyReviewModel.
func (m *WeeklyReviewModel) buildKeyMap() *keymap.KeyMap {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	hasItem := func() bool { return m.getTaskAtCursor() != nil }

	var entries []keymap.Entry

	// --- List navigation: j, k, G, ctrl+u, ctrl+d ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			if m.cursor < m.totalItems-1 {
				m.cursor++
			}
			return m, nil
		},
		CursorUp: func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		},
		JumpToBottom: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			if m.totalItems > 0 {
				m.cursor = m.totalItems - 1
			}
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, m.totalItems, -HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, m.totalItems, HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
	})...)

	// --- Goto group: g prefix + section-focus sub-keys ---
	// gp navigates to project (if on project row) or task (if on task row).
	focusWeeklySection := func(s WeeklyReviewSection) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			return m.toggleSectionFocus(s)
		}
	}
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) { return m, m.JumpToTop() },
		GoToProject: func() (tea.Model, tea.Cmd) {
			if wp := m.getProjectAtCursor(); wp != nil {
				return m, func() tea.Msg { return WeeklyReviewSelectProjectMsg{Project: wp.Project} }
			}
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return WeeklyReviewSelectTaskMsg{Task: task, Project: project} }
		},
		Extra: []keymap.Entry{
			keymap.Bind("P", "Focus Stale Projects section", "Sections", focusWeeklySection(WeeklySectionStaleProjects)),
			keymap.Bind("i", "Focus Inbox Backlog section", "Sections", focusWeeklySection(WeeklySectionInboxBacklog)),
			keymap.Bind("b", "Focus Blocked Work section", "Sections", focusWeeklySection(WeeklySectionBlockedWork)),
			keymap.Bind("o", "Focus Overcommitted section", "Sections", focusWeeklySection(WeeklySectionOvercommitted)),
			keymap.Bind("t", "Focus Stale Tasks section", "Sections", focusWeeklySection(WeeklySectionStaleTasks)),
			keymap.Bind("m", "Focus Priority Mismatch section", "Sections", focusWeeklySection(WeeklySectionPriorityMismatch)),
			keymap.Bind("w", "Focus Upcoming Week section", "Sections", focusWeeklySection(WeeklySectionUpcomingWeek)),
			keymap.Bind("r", "Focus Recurring Overview section", "Sections", focusWeeklySection(WeeklySectionRecurringOverview)),
		},
	})...)

	// --- z prefix: zp (preview), zR (expand all), zM (collapse all) ---
	entries = append(entries, keymap.Prefix("z", "View", "View", []keymap.Entry{
		keymap.Bind("p", "Toggle inline preview", "View", func() (tea.Model, tea.Cmd) {
			m.previewMode = !m.previewMode
			return m, nil
		}),
		keymap.Bind("R", "Expand all sections", "View", func() (tea.Model, tea.Cmd) {
			ExpandAllSections(m.collapsedSections)
			m.calculateNavigation()
			m.clampCursor()
			return m, nil
		}),
		keymap.Bind("M", "Collapse all sections", "View", func() (tea.Model, tea.Cmd) {
			sectionInts := make([]int, len(m.sectionOrder))
			for i, s := range m.sectionOrder {
				sectionInts[i] = int(s)
			}
			CollapseAllSections(m.collapsedSections, sectionInts)
			m.calculateNavigation()
			m.clampCursor()
			return m, nil
		}),
	}))

	// --- Status group: "," prefix, gated on hasItem ---
	statusEntries := keymap.StatusGroup(keymap.StatusDeps{
		ChangeStatus: func(status string) keymap.Action {
			s := models.TaskStatus(status)
			return func() (tea.Model, tea.Cmd) {
				if m.multiSelectMode && m.SelectedCount() > 0 {
					items := m.getSelectedBulkItems()
					count, err := taskOpsApplyBulk(m, items, StatusVerb(s)+" Task",
						func(proj *models.Project, taskID string) bool {
							t := proj.GetTaskByID(taskID)
							if t == nil || t.CanTransitionTo(s) != "" {
								return false
							}
							proj.MutateTask(taskID, func(t *models.Task) { t.ApplyTransition(s) })
							return true
						})
					m.ExitMultiSelect()
					if err != nil {
						m.setStatus(fmt.Sprintf("Error: %v", err))
					} else if count == 0 {
						m.setStatus("No tasks were changed")
					} else {
						m.setStatus(fmt.Sprintf("Changed %d tasks to %s", count, s))
					}
					return m, nil
				}
				model, cmd := taskOpsChangeStatus(m, s)
				if m.err == nil {
					m.trackMutation("status")
				}
				return model, cmd
			}
		},
		HasRecurrence:  nil,
		SkipRecurrence: nil,
		DeferChildren:  nil,
	})
	if len(statusEntries) > 0 && statusEntries[0].PrefixNode != nil {
		statusEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, statusEntries...)

	// --- Priority group: "!" prefix, gated on hasItem ---
	priorityEntries := keymap.PriorityGroup(keymap.PriorityDeps{
		ChangePriority: func(key string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				if m.multiSelectMode && m.SelectedCount() > 0 {
					p, _ := PriorityForPrefixKey(key)
					items := m.getSelectedBulkItems()
					count, err := taskOpsApplyBulk(m, items, "Set Priority",
						func(proj *models.Project, taskID string) bool {
							t := proj.GetTaskByID(taskID)
							if t == nil || t.Priority == p {
								return false
							}
							proj.MutateTask(taskID, func(t *models.Task) { t.Priority = p })
							return true
						})
					m.ExitMultiSelect()
					if err != nil {
						m.setStatus(fmt.Sprintf("Error: %v", err))
					} else if count == 0 {
						m.setStatus("No tasks were changed")
					} else {
						m.setStatus(fmt.Sprintf("Set priority %d on %d tasks", p, count))
					}
					return m, nil
				}
				model, cmd := taskOpsChangePriority(m, key)
				if m.err == nil {
					m.trackMutation("priority")
				}
				return model, cmd
			}
		},
	})
	if len(priorityEntries) > 0 && priorityEntries[0].PrefixNode != nil {
		priorityEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, priorityEntries...)

	// --- Due date group: "d" prefix, gated on hasItem ---
	bulkSetDueDate := func(days int) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.SelectedCount() > 0 {
				dueDate := util.StartOfDay(time.Now().AddDate(0, 0, days))
				items := m.getSelectedBulkItems()
				count, err := taskOpsApplyBulk(m, items, "Set Due Date",
					func(proj *models.Project, taskID string) bool {
						proj.MutateTask(taskID, func(t *models.Task) {
							d := dueDate
							t.DueDate = &d
							t.HasTime = false
						})
						return true
					})
				m.ExitMultiSelect()
				if err != nil {
					m.setStatus(fmt.Sprintf("Error: %v", err))
				} else {
					m.setStatus(fmt.Sprintf("Due date set on %d tasks", count))
				}
				return m, nil
			}
			model, cmd := taskOpsSetQuickDueDate(m, days)
			if m.err == nil {
				m.trackMutation("due_date")
			}
			return model, cmd
		}
	}
	bulkClearDueDate := func() (tea.Model, tea.Cmd) {
		if m.multiSelectMode && m.SelectedCount() > 0 {
			items := m.getSelectedBulkItems()
			count, err := taskOpsApplyBulk(m, items, "Clear Due Date",
				func(proj *models.Project, taskID string) bool {
					proj.MutateTask(taskID, func(t *models.Task) {
						t.DueDate = nil
						t.HasTime = false
					})
					return true
				})
			m.ExitMultiSelect()
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
			} else {
				m.setStatus(fmt.Sprintf("Cleared due date on %d tasks", count))
			}
			return m, nil
		}
		model, cmd := taskOpsClearDueDate(m)
		if m.err == nil {
			m.trackMutation("due_date")
		}
		return model, cmd
	}
	dueDateEntries := keymap.DueDateGroup(keymap.DueDateDeps{
		DueToday:     bulkSetDueDate(0),
		DueTomorrow:  bulkSetDueDate(1),
		DueInAWeek:   bulkSetDueDate(7),
		DueInAMonth:  bulkSetDueDate(30),
		ClearDueDate: bulkClearDueDate,
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := m.getTaskRef()
			if task == nil {
				return m, nil
			}
			return m, func() tea.Msg { return SetDueDateMsg{Task: task, Project: project} }
		},
	})
	if len(dueDateEntries) > 0 && dueDateEntries[0].PrefixNode != nil {
		dueDateEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, dueDateEntries...)

	// --- Snooze group: "s" prefix, gated on hasItem ---
	snoozeDateAction := func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			snoozeDate, _, ok := resolveQuickSnoozeDate(key)
			if !ok {
				return m, nil
			}
			if m.multiSelectMode && m.SelectedCount() > 0 {
				items := m.getSelectedBulkItems()
				count, err := taskOpsApplyBulk(m, items, "Snooze",
					func(proj *models.Project, taskID string) bool {
						proj.MutateTask(taskID, func(t *models.Task) { t.SnoozedUntil = snoozeDate })
						return true
					})
				m.ExitMultiSelect()
				if err != nil {
					m.setStatus(fmt.Sprintf("Error: %v", err))
				} else {
					m.setStatus(fmt.Sprintf("Snoozed %d tasks", count))
				}
				return m, nil
			}
			model, cmd := taskOpsQuickSnooze(m, snoozeDate)
			if m.err == nil {
				m.trackMutation("snooze")
			}
			return model, cmd
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
		snoozeEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, snoozeEntries...)

	// --- Move group: "m" prefix, gated on hasItem ---
	bulkMove := func(section string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode && m.SelectedCount() > 0 {
				items := m.getSelectedBulkItems()
				count, err := taskOpsApplyBulk(m, items, "Move to Section",
					func(proj *models.Project, taskID string) bool {
						t := proj.GetTaskByID(taskID)
						if t == nil || t.InSection(section) {
							return false
						}
						proj.MutateTask(taskID, func(t *models.Task) { t.SetSection(section) })
						return true
					})
				m.ExitMultiSelect()
				if err != nil {
					m.setStatus(fmt.Sprintf("Error: %v", err))
				} else if count == 0 {
					m.setStatus("No tasks were moved")
				} else {
					m.setStatus(fmt.Sprintf("Moved %d tasks to %s", count, section))
				}
				return m, nil
			}
			model, cmd := taskOpsSectionMove(m, section)
			if m.err == nil {
				m.trackMutation("move")
			}
			return model, cmd
		}
	}
	moveEntries := keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: bulkMove,
		MoveToTop:     func() (tea.Model, tea.Cmd) { return m, nil },
		MoveToBottom:  func() (tea.Model, tea.Cmd) { return m, nil },
	})
	if len(moveEntries) > 0 && moveEntries[0].PrefixNode != nil {
		moveEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, moveEntries...)

	// --- Linked task group: "c" prefix, gated on hasItem ---
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
		linkedEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, linkedEntries...)

	// --- Task prefix group: "t" prefix, gated on hasItem ---
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
		DeleteTask: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task != nil {
				m.showDeleteConfirm(task.ShortID)
			}
			return m, nil
		},
	})
	if len(taskPrefixEntries) > 0 && taskPrefixEntries[0].PrefixNode != nil {
		taskPrefixEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, taskPrefixEntries...)

	// --- Stage group: "." prefix, gated on hasItem ---
	stageEntries := keymap.StageGroup(keymap.StageDeps{
		Stages: stages,
		ChangeStage: func(stage string) keymap.Action {
			return func() (tea.Model, tea.Cmd) {
				if m.multiSelectMode && m.SelectedCount() > 0 {
					items := m.getSelectedBulkItems()
					count, err := taskOpsApplyBulk(m, items, "Set Stage",
						func(proj *models.Project, taskID string) bool {
							t := proj.GetTaskByID(taskID)
							if t == nil || t.Stage == stage {
								return false
							}
							proj.MutateTask(taskID, func(t *models.Task) { t.Stage = stage })
							return true
						})
					m.ExitMultiSelect()
					if err != nil {
						m.setStatus(fmt.Sprintf("Error: %v", err))
					} else if count == 0 {
						m.setStatus("No tasks were changed")
					} else {
						m.setStatus(fmt.Sprintf("Set stage on %d tasks", count))
					}
					return m, nil
				}
				return taskOpsChangeStage(m, stage)
			}
		},
	})
	if len(stageEntries) > 0 && stageEntries[0].PrefixNode != nil {
		stageEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, stageEntries...)

	// --- Blocker group: "b" prefix, gated on hasItem ---
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
		blockerEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, blockerEntries...)

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
		DeleteTask: func() (tea.Model, tea.Cmd) {
			task, _ := m.getTaskRef()
			if task != nil {
				m.showDeleteConfirm(task.ShortID)
			}
			return m, nil
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
			model, cmd := taskOpsToggleActive(m)
			if m.err == nil {
				m.trackMutation("active")
			}
			return model, cmd
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

	// --- View-specific direct bindings ---

	// Enter: navigate to project or task
	entries = append(entries,
		keymap.Bind("enter", "Navigate to project/task", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.handleEnter()
		}),
	)

	// Tab: toggle section collapse
	entries = append(entries,
		keymap.Bind("tab", "Collapse/expand section", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.toggleCurrentSection()
		}),
	)

	// Section navigation
	entries = append(entries,
		keymap.Bind("n", "Next section (show all)", "Navigation", func() (tea.Model, tea.Cmd) {
			m.focusedSection = nil
			m.calculateNavigation()
			return m.goToNextSection()
		}),
		keymap.Bind("N", "Previous section (show all)", "Navigation", func() (tea.Model, tea.Cmd) {
			m.focusedSection = nil
			m.calculateNavigation()
			return m.goToPrevSection()
		}),
		keymap.Bind("ctrl+n", "Next unreviewed section", "Review", func() (tea.Model, tea.Cmd) {
			return m.goToNextUnreviewed()
		}),
	)

	// Tab switching
	entries = append(entries,
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			handleTabCycleKey("shift+right", &m.tabs, func() {
				m.focusedSection = nil
				m.scrollOffset = 0
				m.calculateNavigation()
				m.clampCursor()
			})
			return m, nil
		}),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			handleTabCycleKey("shift+left", &m.tabs, func() {
				m.focusedSection = nil
				m.scrollOffset = 0
				m.calculateNavigation()
				m.clampCursor()
			})
			return m, nil
		}),
	)

	// R: mark section reviewed or reset if already complete
	entries = append(entries,
		keymap.Bind("R", "Mark section reviewed / reset", "Review", func() (tea.Model, tea.Cmd) {
			if m.reviewCompleted {
				m.reviewCompleted = false
				m.showingSummary = false
				for k := range m.reviewedSections {
					delete(m.reviewedSections, k)
				}
				m.loadData()
				return m, func() tea.Msg { return ResetWeeklyReviewMsg{} }
			}
			return m.markCurrentSectionReviewed()
		}),
	)

	// Priority keys: 1-5 (direct priority change with mutation tracking)
	for _, k := range []string{"1", "2", "3", "4", "5"} {
		key := k
		entries = append(entries,
			keymap.Bind(key, "Set priority "+key, "Actions", func() (tea.Model, tea.Cmd) {
				model, cmd := taskOpsChangePriority(m, key)
				if m.err == nil {
					m.trackMutation("priority")
				}
				return model, cmd
			}, keymap.AsHidden()),
		)
	}

	// Multi-select
	entries = append(entries,
		keymap.Bind("v", "Toggle multi-select", "Multi-select", func() (tea.Model, tea.Cmd) {
			m.toggleMultiSelectMode()
			return m, nil
		}),
		keymap.Bind("space", "Toggle selection", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.toggleTaskSelection()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("a", "Select all visible", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.selectAllVisible()
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("A", "Deselect all", "Multi-select", func() (tea.Model, tea.Cmd) {
			if m.multiSelectMode {
				m.deselectAll()
			}
			return m, nil
		}, keymap.AsHidden()),
	)

	return &keymap.KeyMap{
		ViewName: "Weekly Review",
		Entries:  entries,
	}
}
