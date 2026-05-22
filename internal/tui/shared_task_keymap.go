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
	builders := []func(TaskKeymapDeps) []keymap.Entry{
		buildStandardStatusEntries,
		buildStandardPriorityEntries,
		buildStandardDueDateEntries,
		buildStandardSnoozeEntries,
		buildStandardMoveEntries,
		buildStandardLinkedEntries,
		buildStandardTaskPrefixEntries,
		buildStandardStageEntries,
		buildStandardBlockerEntries,
		buildStandardCommonActionEntries,
	}

	var entries []keymap.Entry
	for _, build := range builders {
		entries = append(entries, build(deps)...)
	}
	return entries
}

func buildStandardStatusEntries(deps TaskKeymapDeps) []keymap.Entry {
	return gatePrefixEntries(keymap.StatusGroup(keymap.StatusDeps{
		ChangeStatus:   buildStatusChangeAction(deps),
		HasRecurrence:  deps.HasRecurrence,
		SkipRecurrence: deps.SkipRecurrence,
		DeferChildren:  deps.DeferChildren,
	}), deps.HasItem)
}

func buildStandardPriorityEntries(deps TaskKeymapDeps) []keymap.Entry {
	return gatePrefixEntries(keymap.PriorityGroup(keymap.PriorityDeps{
		ChangePriority: buildPriorityChangeAction(deps),
	}), deps.HasItem)
}

func buildStandardDueDateEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	setDueDate := buildDueDateSetAction(deps)
	return gatePrefixEntries(keymap.DueDateGroup(keymap.DueDateDeps{
		DueToday:     setDueDate(0),
		DueTomorrow:  setDueDate(1),
		DueInAWeek:   setDueDate(7),
		DueInAMonth:  setDueDate(30),
		ClearDueDate: buildDueDateClearAction(deps),
		OpenDatePicker: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return SetDueDateMsg{Task: task, Project: project}
		}),
	}), deps.HasItem)
}

func buildStandardSnoozeEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	snoozeDate := buildSnoozeDateAction(deps)
	return gatePrefixEntries(keymap.SnoozeGroup(keymap.SnoozeDeps{
		SnoozeTomorrow:   snoozeDate("t"),
		SnoozeOneWeek:    snoozeDate("w"),
		SnoozeOneMonth:   snoozeDate("m"),
		SnoozeNextMonday: snoozeDate("n"),
		ClearSnooze:      snoozeDate("0"),
		OpenDatePicker: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return SetSnoozeDateMsg{Task: task, Project: project}
		}),
	}), deps.HasItem)
}

func buildStandardMoveEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	return gatePrefixEntries(keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: buildSectionMoveAction(deps),
		MoveToTop:     func() (tea.Model, tea.Cmd) { return v, nil },
		MoveToBottom:  func() (tea.Model, tea.Cmd) { return v, nil },
	}), deps.HasItem)
}

func buildStandardLinkedEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	return gatePrefixEntries(keymap.LinkedTaskGroup(keymap.LinkedTaskDeps{
		CreateBlocker: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeBlocker}
		}),
		CreateDependent: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeDependent}
		}),
		CreateSubtask: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeSubtask}
		}),
		CreateFollowUp: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeFollowUp}
		}),
		NavigateLinked: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowLinkedTaskSelectorMsg{Task: task, Project: project}
		}),
		CrossProjectLink: deps.LinkedCrossProject,
		UnlinkTask:       deps.LinkedUnlink,
	}), deps.HasItem)
}

func buildStandardTaskPrefixEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	return gatePrefixEntries(keymap.TaskPrefixGroup(keymap.TaskPrefixDeps{
		NewTask: taskRefMsgAction(v, false, func(task *models.Task, _ *models.Project) tea.Msg {
			return CreateTaskMsg{ParentTask: task, SkipTemplatePicker: true}
		}),
		NewTaskWithTemplate: taskRefMsgAction(v, false, func(task *models.Task, _ *models.Project) tea.Msg {
			return CreateTaskMsg{ParentTask: task, SkipTemplatePicker: false}
		}),
		RenameTask: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return EditTaskMsg{Task: task, Project: project}
		}),
		ChangeSection: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowSectionPickerMsg{Task: task, Project: project}
		}),
		OpenEditor: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return EditTaskContentMsg{Task: task, Project: project}
		}),
		TogglePin:  func() (tea.Model, tea.Cmd) { return taskOpsTogglePinned(v) },
		ToggleDim:  func() (tea.Model, tea.Cmd) { return taskOpsToggleDimmed(v) },
		ManageTags: buildManageTagsAction(v),
		ViewComments: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowTaskCommentsMsg{Task: task, Project: project}
		}),
		ViewAttachments: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowTaskAttachmentsMsg{Task: task, Project: project}
		}),
		EditChecklist: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowTaskChecklistMsg{Task: task, Project: project}
		}),
		EditDescription: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowDescriptionEditorMsg{Task: task, Project: project}
		}),
		MoveToProject: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project}
		}),
		ViewDecisions: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowTaskDecisionsMsg{Task: task, Project: project}
		}),
		DeleteTask: buildDeleteTaskAction(deps),
	}), deps.HasItem)
}

func buildStandardStageEntries(deps TaskKeymapDeps) []keymap.Entry {
	return gatePrefixEntries(keymap.StageGroup(keymap.StageDeps{
		Stages:      deps.Stages,
		ChangeStage: buildStageChangeAction(deps),
	}), deps.HasItem)
}

func buildStandardBlockerEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	return gatePrefixEntries(keymap.BlockerGroup(keymap.BlockerDeps{
		ViewDependencies: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowDependencyViewerMsg{Task: task, Project: project}
		}),
		ManageDependencies: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ManageDependenciesMsg{Task: task, Project: project}
		}),
	}), deps.HasItem)
}

func buildStandardCommonActionEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	return keymap.CommonTaskActionGroup(keymap.CommonTaskActionDeps{
		ViewTask: taskRefMsgAction(v, true, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowTaskDetailMsg{Task: task, Project: project}
		}),
		SnoozeDatePicker: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return SetSnoozeDateMsg{Task: task, Project: project}
		}),
		MarkDone: buildMarkDoneAction(deps),
		EditTask: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return EditTaskMsg{Task: task, Project: project}
		}),
		ToggleHelp:   deps.ToggleHelp,
		ToggleActive: buildToggleActiveAction(deps),
		EditScore: taskRefMsgAction(v, true, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowScoreEditorMsg{Task: task, Project: project}
		}),
		OpenURL:       deps.OpenURL,
		MoveToProject: deps.MoveToProject,
		QuickNote:     deps.QuickNote,
	})
}

func gatePrefixEntries(entries []keymap.Entry, gate func() bool) []keymap.Entry {
	if len(entries) > 0 && entries[0].PrefixNode != nil {
		entries[0].PrefixNode.Gate = gate
	}
	return entries
}

func buildStatusChangeAction(deps TaskKeymapDeps) func(string) keymap.Action {
	if deps.StatusChange != nil {
		return deps.StatusChange
	}
	return func(status string) keymap.Action {
		s := models.TaskStatus(status)
		return func() (tea.Model, tea.Cmd) { return taskOpsChangeStatus(deps.View, s) }
	}
}

func buildPriorityChangeAction(deps TaskKeymapDeps) func(string) keymap.Action {
	if deps.PriorityChange != nil {
		return deps.PriorityChange
	}
	return func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) { return taskOpsChangePriority(deps.View, key) }
	}
}

func buildDueDateSetAction(deps TaskKeymapDeps) func(int) keymap.Action {
	if deps.DueDateSet != nil {
		return deps.DueDateSet
	}
	return func(days int) keymap.Action {
		return func() (tea.Model, tea.Cmd) { return taskOpsSetQuickDueDate(deps.View, days) }
	}
}

func buildDueDateClearAction(deps TaskKeymapDeps) keymap.Action {
	if deps.DueDateClear != nil {
		return deps.DueDateClear
	}
	return func() (tea.Model, tea.Cmd) { return taskOpsClearDueDate(deps.View) }
}

func buildSnoozeDateAction(deps TaskKeymapDeps) func(string) keymap.Action {
	if deps.SnoozeDate != nil {
		return deps.SnoozeDate
	}
	return func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) {
			snoozeDate, _, ok := resolveQuickSnoozeDate(key)
			if !ok {
				return deps.View, nil
			}
			return taskOpsQuickSnooze(deps.View, snoozeDate)
		}
	}
}

func buildSectionMoveAction(deps TaskKeymapDeps) func(string) keymap.Action {
	if deps.SectionMove != nil {
		return deps.SectionMove
	}
	return func(section string) keymap.Action {
		return func() (tea.Model, tea.Cmd) { return taskOpsSectionMove(deps.View, section) }
	}
}

func buildStageChangeAction(deps TaskKeymapDeps) func(string) keymap.Action {
	if deps.StageChange != nil {
		return deps.StageChange
	}
	return func(stage string) keymap.Action {
		return func() (tea.Model, tea.Cmd) { return taskOpsChangeStage(deps.View, stage) }
	}
}

func buildDeleteTaskAction(deps TaskKeymapDeps) keymap.Action {
	if deps.DeleteTask != nil {
		return deps.DeleteTask
	}
	return func() (tea.Model, tea.Cmd) { return taskOpsDeleteTask(deps.View) }
}

func buildMarkDoneAction(deps TaskKeymapDeps) keymap.Action {
	if deps.MarkDone != nil {
		return deps.MarkDone
	}
	return func() (tea.Model, tea.Cmd) { return taskOpsChangeStatus(deps.View, models.StatusDone) }
}

func buildToggleActiveAction(deps TaskKeymapDeps) keymap.Action {
	if deps.ToggleActive != nil {
		return deps.ToggleActive
	}
	return func() (tea.Model, tea.Cmd) { return taskOpsToggleActive(deps.View) }
}

func taskRefAction(
	v taskOpsView,
	requireProject bool,
	fn func(task *models.Task, project *models.Project) (tea.Model, tea.Cmd),
) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		task, project := v.getTaskRef()
		if task == nil || (requireProject && project == nil) {
			return v, nil
		}
		return fn(task, project)
	}
}

func taskRefMsgAction(v taskOpsView, requireProject bool, msg func(task *models.Task, project *models.Project) tea.Msg) keymap.Action {
	return taskRefAction(v, requireProject, func(task *models.Task, project *models.Project) (tea.Model, tea.Cmd) {
		return v, func() tea.Msg { return msg(task, project) }
	})
}

func buildManageTagsAction(v taskOpsView) keymap.Action {
	return taskRefAction(v, false, func(task *models.Task, project *models.Project) (tea.Model, tea.Cmd) {
		tags := append([]string{}, task.Tags...)
		return v, func() tea.Msg {
			return ShowTagSelectorMsg{Task: task, Project: project, CurrentTags: tags}
		}
	})
}
