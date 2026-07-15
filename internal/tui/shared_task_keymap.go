package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// taskActionView is the concrete-model seam every standard task-action keymap
// binds against. It satisfies both taskOpsView (message/ref helpers) and
// workflowOpsView (coordinator-backed mutations), so each non-overridden action
// routes through the wf* helpers that commit via the application-layer
// TaskItemCoordinator. All list/board views satisfy both interfaces.
type taskActionView interface {
	taskOpsView
	workflowOpsView
}

// TaskKeymapDeps configures buildStandardTaskActionEntries.
//
// Required fields: View, HasItem, ToggleHelp.
//
// Bulk/override closures (StatusChange, PriorityChange, etc.) are nil-safe:
// nil means "use the standard single-item wf* helper". Views that need
// bulk multi-select or additional side effects (e.g. WeeklyReview mutation
// tracking) provide their own closures.
type TaskKeymapDeps struct {
	// View is the concrete model. Every non-overridden standard action routes
	// through the coordinator-backed wf* helpers.
	View taskActionView
	// HasItem reports whether a task is currently under the cursor.
	HasItem func() bool
	// ToggleHelp opens the help panel; view-specific (needs viewID/viewName).
	ToggleHelp keymap.Action
	// SelectedTaskRefs returns the active multi-selection as task/project refs.
	// nil = this view does not expose task multi-select to shared actions.
	SelectedTaskRefs func() []TaskProjectRef

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

	// Per-group overrides — nil uses the standard wf* helper.
	NewTask             keymap.Action
	NewTaskWithTemplate keymap.Action
	StateChange         func(state string) keymap.Action
	PriorityChange      func(key string) keymap.Action
	DueDateSet          func(days int) keymap.Action
	DueDateClear        keymap.Action
	SnoozeDate          func(key string) keymap.Action
	SectionMove         func(section string) keymap.Action
	MarkDone            keymap.Action
	TogglePin           keymap.Action
	ToggleDim           keymap.Action
}

// buildStandardTaskActionEntries returns the standard task-action keymap entries
// for the groups: Status, Priority, DueDate, Snooze, Move, LinkedTask, TaskPrefix,
// Blocker, and CommonTaskAction — wired through wf* helpers.
//
// Each prefix group is gated on deps.HasItem or an active selected-task target.
// The caller appends the returned entries alongside view-specific navigation,
// filter, and sort groups.
func buildStandardTaskActionEntries(deps TaskKeymapDeps) []keymap.Entry {
	builders := []func(TaskKeymapDeps) []keymap.Entry{
		buildStandardStatusEntries,
		buildStandardPriorityEntries,
		buildStandardDueDateEntries,
		buildStandardSnoozeEntries,
		buildStandardMoveEntries,
		buildStandardLinkedEntries,
		buildStandardTaskPrefixEntries,
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
		ChangeState:    buildStateChangeAction(deps),
		HasRecurrence:  deps.HasRecurrence,
		SkipRecurrence: deps.SkipRecurrence,
		DeferChildren:  deps.DeferChildren,
	}), taskActionTargetGate(deps))
}

func buildStandardPriorityEntries(deps TaskKeymapDeps) []keymap.Entry {
	return gatePrefixEntries(keymap.PriorityGroup(keymap.PriorityDeps{
		ChangePriority: buildPriorityChangeAction(deps),
	}), taskActionTargetGate(deps))
}

func buildStandardDueDateEntries(deps TaskKeymapDeps) []keymap.Entry {
	setDueDate := buildDueDateSetAction(deps)
	return gatePrefixEntries(keymap.DueDateGroup(keymap.DueDateDeps{
		DueToday:     setDueDate(0),
		DueTomorrow:  setDueDate(1),
		DueInAWeek:   setDueDate(7),
		DueInAMonth:  setDueDate(30),
		ClearDueDate: buildDueDateClearAction(deps),
		OpenDatePicker: taskRefsOrRefMsgAction(deps, func(refs []TaskProjectRef) tea.Msg {
			return SetDueDateMsg{TaskRefs: refs}
		}, false, func(task *models.Task, project *models.Project) tea.Msg {
			return SetDueDateMsg{Task: task, Project: project}
		}),
	}), taskActionTargetGate(deps))
}

func buildStandardSnoozeEntries(deps TaskKeymapDeps) []keymap.Entry {
	snoozeDate := buildSnoozeDateAction(deps)
	return gatePrefixEntries(keymap.SnoozeGroup(keymap.SnoozeDeps{
		SnoozeTomorrow:   snoozeDate("t"),
		SnoozeOneWeek:    snoozeDate("w"),
		SnoozeOneMonth:   snoozeDate("m"),
		SnoozeNextMonday: snoozeDate("n"),
		ClearSnooze:      snoozeDate("0"),
		OpenDatePicker: taskRefsOrRefMsgAction(deps, func(refs []TaskProjectRef) tea.Msg {
			return SetSnoozeDateMsg{TaskRefs: refs}
		}, false, func(task *models.Task, project *models.Project) tea.Msg {
			return SetSnoozeDateMsg{Task: task, Project: project}
		}),
	}), taskActionTargetGate(deps))
}

func buildStandardMoveEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	return gatePrefixEntries(keymap.MoveGroup(keymap.MoveDeps{
		MoveToSection: buildSectionMoveAction(deps),
		MoveToTop:     func() (tea.Model, tea.Cmd) { return v, nil },
		MoveToBottom:  func() (tea.Model, tea.Cmd) { return v, nil },
	}), taskActionTargetGate(deps))
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
	}), taskActionTargetGate(deps))
}

func buildStandardTaskPrefixEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	return gatePrefixEntries(keymap.TaskPrefixGroup(keymap.TaskPrefixDeps{
		NewTask:             buildNewTaskAction(deps, true),
		NewTaskWithTemplate: buildNewTaskAction(deps, false),
		RenameTask: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return EditTaskMsg{Task: task, Project: project}
		}),
		ChangeSection: buildSectionPickerAction(deps),
		OpenEditor: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return EditTaskContentMsg{Task: task, Project: project}
		}),
		TogglePin:  buildTogglePinAction(deps),
		ToggleDim:  buildToggleDimAction(deps),
		ManageTags: buildManageTagsAction(deps),
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
		ParkTask: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowParkInputMsg{Task: task, Project: project}
		}),
		DeleteTask: buildDeleteTaskAction(deps),
	}), taskActionTargetGate(deps))
}

func buildNewTaskAction(deps TaskKeymapDeps, skipTemplatePicker bool) keymap.Action {
	if skipTemplatePicker && deps.NewTask != nil {
		return deps.NewTask
	}
	if !skipTemplatePicker && deps.NewTaskWithTemplate != nil {
		return deps.NewTaskWithTemplate
	}
	return taskRefMsgAction(deps.View, false, func(task *models.Task, _ *models.Project) tea.Msg {
		return CreateTaskMsg{ParentTask: task, SkipTemplatePicker: skipTemplatePicker}
	})
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
	}), taskActionTargetGate(deps))
}

func buildStandardCommonActionEntries(deps TaskKeymapDeps) []keymap.Entry {
	v := deps.View
	return keymap.CommonTaskActionGroup(keymap.CommonTaskActionDeps{
		ViewTask: taskRefMsgAction(v, true, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowTaskDetailMsg{Task: task, Project: project}
		}),
		SnoozeDatePicker: taskRefsOrRefMsgAction(deps, func(refs []TaskProjectRef) tea.Msg {
			return SetSnoozeDateMsg{TaskRefs: refs}
		}, false, func(task *models.Task, project *models.Project) tea.Msg {
			return SetSnoozeDateMsg{Task: task, Project: project}
		}),
		MarkDone: buildMarkDoneAction(deps),
		EditTask: taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return EditTaskMsg{Task: task, Project: project}
		}),
		ToggleHelp: deps.ToggleHelp,
		EditScore: taskRefMsgAction(v, true, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowScoreEditorMsg{Task: task, Project: project}
		}),
		OpenURL:       deps.OpenURL,
		MoveToProject: deps.MoveToProject,
		QuickNote:     deps.QuickNote,
	})
}

func taskActionTargetGate(deps TaskKeymapDeps) func() bool {
	return func() bool {
		if deps.HasItem != nil && deps.HasItem() {
			return true
		}
		return len(selectedTaskRefs(deps)) > 0
	}
}

func gatePrefixEntries(entries []keymap.Entry, gate func() bool) []keymap.Entry {
	if len(entries) > 0 && entries[0].PrefixNode != nil {
		entries[0].PrefixNode.Gate = gate
	}
	return entries
}

func buildStateChangeAction(deps TaskKeymapDeps) func(string) keymap.Action {
	if deps.StateChange != nil {
		return deps.StateChange
	}
	return func(state string) keymap.Action {
		s := models.WorkflowState(state)
		return func() (tea.Model, tea.Cmd) { return deps.View, wfChangeState(deps.View, s) }
	}
}

func buildPriorityChangeAction(deps TaskKeymapDeps) func(string) keymap.Action {
	if deps.PriorityChange != nil {
		return deps.PriorityChange
	}
	return func(key string) keymap.Action {
		return func() (tea.Model, tea.Cmd) { return deps.View, wfChangePriority(deps.View, key) }
	}
}

func buildDueDateSetAction(deps TaskKeymapDeps) func(int) keymap.Action {
	if deps.DueDateSet != nil {
		return deps.DueDateSet
	}
	return func(days int) keymap.Action {
		return func() (tea.Model, tea.Cmd) { return deps.View, wfSetQuickDueDate(deps.View, days) }
	}
}

func buildDueDateClearAction(deps TaskKeymapDeps) keymap.Action {
	if deps.DueDateClear != nil {
		return deps.DueDateClear
	}
	return func() (tea.Model, tea.Cmd) { return deps.View, wfClearDueDate(deps.View) }
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
			return deps.View, wfQuickSnooze(deps.View, snoozeDate)
		}
	}
}

func buildSectionMoveAction(deps TaskKeymapDeps) func(string) keymap.Action {
	if deps.SectionMove != nil {
		return deps.SectionMove
	}
	return func(section string) keymap.Action {
		return func() (tea.Model, tea.Cmd) { return deps.View, wfSectionMove(deps.View, section) }
	}
}

func buildDeleteTaskAction(deps TaskKeymapDeps) keymap.Action {
	if deps.DeleteTask != nil {
		return deps.DeleteTask
	}
	return func() (tea.Model, tea.Cmd) {
		return deps.View, wfDeleteTask(deps.View, deps.View.getRuntime(), deps.View.getConfigManager())
	}
}

func buildMarkDoneAction(deps TaskKeymapDeps) keymap.Action {
	if deps.MarkDone != nil {
		return deps.MarkDone
	}
	return func() (tea.Model, tea.Cmd) { return deps.View, wfChangeState(deps.View, models.StateDone) }
}

func buildTogglePinAction(deps TaskKeymapDeps) keymap.Action {
	if deps.TogglePin != nil {
		return deps.TogglePin
	}
	return func() (tea.Model, tea.Cmd) { return deps.View, wfTogglePin(deps.View) }
}

func buildToggleDimAction(deps TaskKeymapDeps) keymap.Action {
	if deps.ToggleDim != nil {
		return deps.ToggleDim
	}
	return func() (tea.Model, tea.Cmd) { return deps.View, wfToggleDim(deps.View) }
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

func taskRefsOrRefMsgAction(
	deps TaskKeymapDeps,
	bulkMsg func(refs []TaskProjectRef) tea.Msg,
	requireProject bool,
	singleMsg func(task *models.Task, project *models.Project) tea.Msg,
) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		if refs := selectedTaskRefs(deps); len(refs) > 0 {
			return deps.View, func() tea.Msg { return bulkMsg(refs) }
		}
		return taskRefMsgAction(deps.View, requireProject, singleMsg)()
	}
}

func selectedTaskRefs(deps TaskKeymapDeps) []TaskProjectRef {
	if deps.SelectedTaskRefs == nil {
		return nil
	}
	return validTaskProjectRefs(deps.SelectedTaskRefs())
}

func buildSectionPickerAction(deps TaskKeymapDeps) keymap.Action {
	v := deps.View
	return func() (tea.Model, tea.Cmd) {
		if refs := selectedTaskRefs(deps); len(refs) > 0 {
			project, tasks, ok := sameProjectTaskRefs(refs)
			if !ok {
				v.setStatus("bulk section move not supported across projects")
				return v, nil
			}
			return v, func() tea.Msg {
				return ShowBulkSectionPickerMsg{Tasks: tasks, Project: project}
			}
		}
		return taskRefMsgAction(v, false, func(task *models.Task, project *models.Project) tea.Msg {
			return ShowSectionPickerMsg{Task: task, Project: project}
		})()
	}
}

func sameProjectTaskRefs(refs []TaskProjectRef) (*models.Project, []*models.Task, bool) {
	refs = validTaskProjectRefs(refs)
	if len(refs) == 0 {
		return nil, nil, false
	}
	project := refs[0].Project
	tasks := make([]*models.Task, 0, len(refs))
	for _, ref := range refs {
		if ref.Project == nil || project == nil || ref.Project.ID != project.ID {
			return nil, nil, false
		}
		tasks = append(tasks, ref.Task)
	}
	return project, tasks, true
}

func buildManageTagsAction(deps TaskKeymapDeps) keymap.Action {
	v := deps.View
	return func() (tea.Model, tea.Cmd) {
		if refs := selectedTaskRefs(deps); len(refs) > 0 {
			return v, func() tea.Msg { return ShowBulkTagSelectorMsg{TaskRefs: refs} }
		}
		return taskRefAction(v, false, func(task *models.Task, project *models.Project) (tea.Model, tea.Cmd) {
			tags := append([]string{}, task.Tags...)
			return v, func() tea.Msg {
				return ShowTagSelectorMsg{Task: task, Project: project, CurrentTags: tags}
			}
		})()
	}
}
