package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// TaskKeymapDeps configures buildStandardTaskActionEntries.
//
// Required fields: View, HasItem, Stages, ToggleHelp.
//
// Bulk/override closures (StatusChange, PriorityChange, etc.) are nil-safe:
// nil means "use the standard single-item taskOps* helper". Views that need
// bulk multi-select or additional side effects (e.g. WeeklyReview mutation
// tracking) provide their own closures.
type TaskKeymapDeps struct {
	// View is the taskOpsView implementation (the concrete model).
	View taskOpsView
	// HasItem reports whether a task is currently under the cursor.
	HasItem func() bool
	// Stages is the list of workflow stage names from config.
	Stages []string
	// ToggleHelp opens the help panel; view-specific (needs viewID/viewName).
	ToggleHelp keymap.Action

	// DeleteTask shows a confirmation dialog before deletion.
	// nil = call taskOpsDeleteTask directly (no dialog).
	DeleteTask keymap.Action

	// Status recurrence — nil skips recurrence bindings (,k and ,D).
	HasRecurrence  func() bool
	SkipRecurrence keymap.Action
	DeferChildren  []keymap.Entry

	// Linked task optional extras.
	LinkedCrossProject keymap.Action // nil = skip cx binding
	LinkedUnlink       keymap.Action // nil = skip cu binding

	// CommonTaskAction optional extras.
	OpenURL       keymap.Action // nil = skip O binding
	MoveToProject keymap.Action // nil = skip ctrl+b binding
	QuickNote     keymap.Action // nil = skip ctrl+. binding

	// Per-group override closures — nil uses the standard taskOps* helper.
	StatusChange   func(status string) keymap.Action
	PriorityChange func(key string) keymap.Action
	DueDateSet     func(days int) keymap.Action
	DueDateClear   keymap.Action
	SnoozeDate     func(key string) keymap.Action
	SectionMove    func(section string) keymap.Action
	StageChange    func(stage string) keymap.Action
	MarkDone       keymap.Action
	ToggleActive   keymap.Action
}

// buildStandardTaskActionEntries returns the standard task-action keymap entries
// for the groups: Status, Priority, DueDate, Snooze, Move, LinkedTask, TaskPrefix,
// Stage, Blocker, and CommonTaskAction — wired through taskOps* helpers.
//
// Each prefix group is gated on deps.HasItem. The caller appends the returned
// entries alongside view-specific navigation, filter, and sort groups.
func buildStandardTaskActionEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	hasItem := deps.HasItem
	var entries []keymap.Entry

	// --- Status group: "," prefix ---
	changeStatus := func(status string) keymap.Action {
		if deps.StatusChange != nil {
			return deps.StatusChange(status)
		}
		s := models.TaskStatus(status)
		return func() (tea.Model, tea.Cmd) { return taskOpsChangeStatus(v, s) }
	}
	statusEntries := keymap.StatusGroup(keymap.StatusDeps{
		ChangeStatus:   changeStatus,
		HasRecurrence:  deps.HasRecurrence,
		SkipRecurrence: deps.SkipRecurrence,
		DeferChildren:  deps.DeferChildren,
	})
	if len(statusEntries) > 0 && statusEntries[0].PrefixNode != nil {
		statusEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, statusEntries...)

	// --- Priority group: "!" prefix ---
	changePriority := func(key string) keymap.Action {
		if deps.PriorityChange != nil {
			return deps.PriorityChange(key)
		}
		return func() (tea.Model, tea.Cmd) { return taskOpsChangePriority(v, key) }
	}
	priorityEntries := keymap.PriorityGroup(keymap.PriorityDeps{
		ChangePriority: changePriority,
	})
	if len(priorityEntries) > 0 && priorityEntries[0].PrefixNode != nil {
		priorityEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, priorityEntries...)

	// --- Due date group: "d" prefix ---
	setDueDate := func(days int) keymap.Action {
		if deps.DueDateSet != nil {
			return deps.DueDateSet(days)
		}
		return func() (tea.Model, tea.Cmd) { return taskOpsSetQuickDueDate(v, days) }
	}
	clearDueDate := func() (tea.Model, tea.Cmd) {
		if deps.DueDateClear != nil {
			return deps.DueDateClear()
		}
		return taskOpsClearDueDate(v)
	}
	dueDateEntries := keymap.DueDateGroup(keymap.DueDateDeps{
		DueToday:    setDueDate(0),
		DueTomorrow: setDueDate(1),
		DueInAWeek:  setDueDate(7),
		DueInAMonth: setDueDate(30),
		ClearDueDate: clearDueDate,
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return SetDueDateMsg{Task: task, Project: project} }
		},
	})
	if len(dueDateEntries) > 0 && dueDateEntries[0].PrefixNode != nil {
		dueDateEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, dueDateEntries...)

	// --- Snooze group: "s" prefix ---
	snoozeDate := func(key string) keymap.Action {
		if deps.SnoozeDate != nil {
			return deps.SnoozeDate(key)
		}
		return func() (tea.Model, tea.Cmd) {
			sd, _, ok := resolveQuickSnoozeDate(key)
			if !ok {
				return v, nil
			}
			return taskOpsQuickSnooze(v, sd)
		}
	}
	snoozeEntries := keymap.SnoozeGroup(keymap.SnoozeDeps{
		SnoozeTomorrow:   snoozeDate("t"),
		SnoozeOneWeek:    snoozeDate("w"),
		SnoozeOneMonth:   snoozeDate("m"),
		SnoozeNextMonday: snoozeDate("n"),
		ClearSnooze:      snoozeDate("0"),
		OpenDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return SetSnoozeDateMsg{Task: task, Project: project} }
		},
	})
	if len(snoozeEntries) > 0 && snoozeEntries[0].PrefixNode != nil {
		snoozeEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, snoozeEntries...)

	// --- Move group: "m" prefix ---
	sectionMove := func(section string) keymap.Action {
		if deps.SectionMove != nil {
			return deps.SectionMove(section)
		}
		return func() (tea.Model, tea.Cmd) { return taskOpsSectionMove(v, section) }
	}
	moveEntries := keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: sectionMove,
		MoveToTop:     func() (tea.Model, tea.Cmd) { return v, nil },
		MoveToBottom:  func() (tea.Model, tea.Cmd) { return v, nil },
	})
	if len(moveEntries) > 0 && moveEntries[0].PrefixNode != nil {
		moveEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, moveEntries...)

	// --- Linked task group: "c" prefix ---
	linkedEntries := keymap.LinkedTaskGroup(keymap.LinkedTaskDeps{
		CreateBlocker: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeBlocker}
			}
		},
		CreateDependent: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeDependent}
			}
		},
		CreateSubtask: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeSubtask}
			}
		},
		CreateFollowUp: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg {
				return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeFollowUp}
			}
		},
		NavigateLinked: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowLinkedTaskSelectorMsg{Task: task, Project: project} }
		},
		CrossProjectLink: deps.LinkedCrossProject,
		UnlinkTask:       deps.LinkedUnlink,
	})
	if len(linkedEntries) > 0 && linkedEntries[0].PrefixNode != nil {
		linkedEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, linkedEntries...)

	// --- Task prefix group: "t" prefix ---
	deleteTask := func() (tea.Model, tea.Cmd) {
		if deps.DeleteTask != nil {
			return deps.DeleteTask()
		}
		return taskOpsDeleteTask(v)
	}
	taskPrefixEntries := keymap.TaskPrefixGroup(keymap.TaskPrefixDeps{
		NewTask: func() (tea.Model, tea.Cmd) {
			task, _ := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return CreateTaskMsg{ParentTask: task, SkipTemplatePicker: true} }
		},
		NewTaskWithTemplate: func() (tea.Model, tea.Cmd) {
			task, _ := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return CreateTaskMsg{ParentTask: task, SkipTemplatePicker: false} }
		},
		RenameTask: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return EditTaskMsg{Task: task, Project: project} }
		},
		ChangeSection: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowSectionPickerMsg{Task: task, Project: project} }
		},
		OpenEditor: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return EditTaskContentMsg{Task: task, Project: project} }
		},
		TogglePin: func() (tea.Model, tea.Cmd) { return taskOpsTogglePinned(v) },
		ToggleDim: func() (tea.Model, tea.Cmd) { return taskOpsToggleDimmed(v) },
		ManageTags: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			tags := append([]string{}, task.Tags...)
			return v, func() tea.Msg {
				return ShowTagSelectorMsg{Task: task, Project: project, CurrentTags: tags}
			}
		},
		ViewComments: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowTaskCommentsMsg{Task: task, Project: project} }
		},
		ViewAttachments: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowTaskAttachmentsMsg{Task: task, Project: project} }
		},
		EditChecklist: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowTaskChecklistMsg{Task: task, Project: project} }
		},
		EditDescription: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowDescriptionEditorMsg{Task: task, Project: project} }
		},
		MoveToProject: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project} }
		},
		ViewDecisions: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowTaskDecisionsMsg{Task: task, Project: project} }
		},
		DeleteTask: deleteTask,
	})
	if len(taskPrefixEntries) > 0 && taskPrefixEntries[0].PrefixNode != nil {
		taskPrefixEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, taskPrefixEntries...)

	// --- Stage group: "." prefix ---
	stageChange := func(stage string) keymap.Action {
		if deps.StageChange != nil {
			return deps.StageChange(stage)
		}
		return func() (tea.Model, tea.Cmd) { return taskOpsChangeStage(v, stage) }
	}
	stageEntries := keymap.StageGroup(keymap.StageDeps{
		Stages:      deps.Stages,
		ChangeStage: stageChange,
	})
	if len(stageEntries) > 0 && stageEntries[0].PrefixNode != nil {
		stageEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, stageEntries...)

	// --- Blocker group: "b" prefix ---
	blockerEntries := keymap.BlockerGroup(keymap.BlockerDeps{
		ViewDependencies: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowDependencyViewerMsg{Task: task, Project: project} }
		},
		ManageDependencies: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ManageDependenciesMsg{Task: task, Project: project} }
		},
	})
	if len(blockerEntries) > 0 && blockerEntries[0].PrefixNode != nil {
		blockerEntries[0].PrefixNode.Gate = hasItem
	}
	entries = append(entries, blockerEntries...)

	// --- Common task actions: o, Z, D, e, ?, *, S, O, ctrl+b, ctrl+. ---
	markDone := func() (tea.Model, tea.Cmd) {
		if deps.MarkDone != nil {
			return deps.MarkDone()
		}
		return taskOpsChangeStatus(v, models.StatusDone)
	}
	toggleActive := func() (tea.Model, tea.Cmd) {
		if deps.ToggleActive != nil {
			return deps.ToggleActive()
		}
		return taskOpsToggleActive(v)
	}
	entries = append(entries, keymap.CommonTaskActionGroup(keymap.CommonTaskActionDeps{
		ViewTask: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil || project == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowTaskDetailMsg{Task: task, Project: project} }
		},
		SnoozeDatePicker: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return SetSnoozeDateMsg{Task: task, Project: project} }
		},
		MarkDone: markDone,
		EditTask: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil {
				return v, nil
			}
			return v, func() tea.Msg { return EditTaskMsg{Task: task, Project: project} }
		},
		ToggleHelp:   deps.ToggleHelp,
		ToggleActive: toggleActive,
		EditScore: func() (tea.Model, tea.Cmd) {
			task, project := v.getTaskRef()
			if task == nil || project == nil {
				return v, nil
			}
			return v, func() tea.Msg { return ShowScoreEditorMsg{Task: task, Project: project} }
		},
		OpenURL:       deps.OpenURL,
		MoveToProject: deps.MoveToProject,
		QuickNote:     deps.QuickNote,
	})...)

	return entries
}
