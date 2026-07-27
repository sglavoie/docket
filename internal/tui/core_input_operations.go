package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func coreOutlinerApplicability(ctx CommandContext) bool {
	return ctx.View == ViewCrossProjectOutliner
}

func coreInputOperationDescriptors() []OperationDescriptor {
	core := func(
		id OperationID,
		label string,
		category OperationCategory,
		cardinality TargetCardinality,
		risk OperationRisk,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                   id,
			Label:                label,
			Aliases:              aliases,
			Category:             category,
			TargetCardinality:    cardinality,
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:        coreOutlinerApplicability,
			Risk:                 risk,
		}
	}
	return []OperationDescriptor{
		core(operationCoreCapture, "Capture", OperationCategoryTask, TargetCardinalityNone, OperationRiskMutation, "add task", "inbox capture"),
		core(operationCoreEdit, "Edit Task", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "modify task"),
		core(operationCoreStart, "Start Task", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "in progress"),
		core(operationCoreDone, "Complete Task", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "done", "finish task"),
		core(operationCoreBacklog, "Move to Backlog", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "ready"),
		core(operationCoreToday, "Toggle Today", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "daily plan"),
		core(operationCoreNext, "Toggle Next", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "on deck"),
		core(operationCoreActions, "Task Actions", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskReadOnly, "action sheet"),
		core(operationCoreCursorDown, "Move Cursor Down", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorUp, "Move Cursor Up", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCollapseParent, "Collapse or Go to Parent", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreExpand, "Expand", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreReorderDown, "Reorder Down", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(operationCoreReorderUp, "Reorder Up", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(operationCoreDetail, "Open Task Detail", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskReadOnly, "task detail"),
		core(operationCoreVisibleSearch, "Search Visible Tasks", OperationCategoryFilter, TargetCardinalityNone, OperationRiskReadOnly, "filter visible list"),
		core(operationCoreFinder, "Global Finder", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "find anything"),
		core(operationCoreModes, "Choose Mode", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "modes"),
		core(operationCoreScope, "Choose Scope", OperationCategoryScope, TargetCardinalityNone, OperationRiskReadOnly, "projects", "contexts"),
		core(operationCoreMultiSelect, "Toggle Multi-select", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "visual selection"),
		core(operationCoreNotes, "Notes", OperationCategoryNote, TargetCardinalityNone, OperationRiskReadOnly, "scratch notes"),
		core(operationCoreWeeklyReview, "Weekly Review", OperationCategoryReview, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreTags, "Tags", OperationCategoryTag, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreNotifications, "Notifications", OperationCategoryNotification, TargetCardinalityNone, OperationRiskReadOnly, "alerts"),
		core(operationCoreUndo, "Undo", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation),
		core(operationCoreRedo, "Redo", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation),
		core(operationCoreBack, "Back", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreReset, "Reset to All Projects", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "root", "global all"),
		core(operationCoreHelp, "Help", OperationCategorySystem, TargetCardinalityNone, OperationRiskReadOnly, "keyboard shortcuts"),
		core(operationCoreQuit, "Quit", OperationCategorySystem, TargetCardinalityNone, OperationRiskReadOnly, "exit"),
		core(operationCoreNextWorkMode, "Next Work Mode", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCorePreviousWorkMode, "Previous Work Mode", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly),
	}
}

// registerCoreInputOperations stages native final-grammar operations without
// adding them to the retained legacy Command palette.
func (m *AppModel) registerCoreInputOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	for _, descriptor := range coreInputOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities:     NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:            coreOutlinerApplicability,
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
			Handler:                  m.executeCoreInputOperation,
		}); err != nil {
			return err
		}
	}
	return nil
}

//nolint:gocyclo // The direct-key dispatcher keeps the final key grammar in one auditable switch.
func (m *AppModel) executeCoreInputOperation(invocation OperationInvocation) OperationHandlerResult {
	var message tea.Msg
	//nolint:exhaustive // Only core input operations are registered with this handler.
	switch invocation.Descriptor.ID {
	case operationCoreCapture:
		message = m.coreCaptureMessage()
	case operationCoreEdit:
		task, project := m.outlinerTarget()
		if task != nil {
			message = EditTaskMsg{Task: task, Project: project}
		}
	case operationCoreStart:
		message = commandMessage(m.coreTransitionTask(models.StateInProgress))
	case operationCoreDone:
		message = commandMessage(m.coreTransitionTask(models.StateDone))
	case operationCoreBacklog:
		message = commandMessage(m.coreMoveToBacklog())
	case operationCoreToday:
		message = commandMessage(m.toggleDailyPlan())
	case operationCoreNext:
		message = commandMessage(m.toggleQueue())
	case operationCoreActions:
		if out, _, ok := m.outlinerPrimary(); ok && out.multiSelectMode {
			out.toggleCursorSelection()
		} else if err := m.openTaskActionSheet(invocation.Context); err != nil {
			m.setInfoStatus(err.Error())
		}
	case operationCoreCursorDown:
		message = commandMessage(m.coreMoveCursor(1))
	case operationCoreCursorUp:
		message = commandMessage(m.coreMoveCursor(-1))
	case operationCoreCollapseParent:
		if out, _, ok := m.outlinerPrimary(); ok {
			out.collapseOrMoveToParent()
		}
	case operationCoreExpand:
		if out, _, ok := m.outlinerPrimary(); ok {
			out.expandAtCursor()
		}
	case operationCoreReorderDown:
		message = commandMessage(m.coreReorder(1))
	case operationCoreReorderUp:
		message = commandMessage(m.coreReorder(-1))
	case operationCoreDetail:
		if out, _, ok := m.outlinerPrimary(); ok && out.multiSelectMode {
			if err := m.openTaskActionSheet(invocation.Context); err != nil {
				out.setStatus(err.Error())
			}
		} else {
			message = m.coreTaskDetailMessage()
		}
	case operationCoreVisibleSearch:
		if out, _, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.ActivateSearch())
		}
	case operationCoreFinder:
		message = ShowFinderMsg{}
	case operationCoreModes:
		message = ShowModeListMsg{}
	case operationCoreScope:
		message = ShowContextHubMsg{}
	case operationCoreMultiSelect:
		if out, _, ok := m.outlinerPrimary(); ok {
			out.toggleMultiSelectMode()
		}
	case operationCoreNotes:
		message = m.coreNotesMessage()
	case operationCoreWeeklyReview:
		message = ShowWeeklyReviewMsg{
			ProjectSet: ActiveProjectSetScope(invocation.Context.ScopeProjectIDs),
		}
	case operationCoreTags:
		message = m.coreTagsMessage(invocation.Context.ScopeProjectIDs)
	case operationCoreNotifications:
		message = ShowNotificationCenterMsg{}
	case operationCoreUndo:
		message = commandMessage(m.coreUndo())
	case operationCoreRedo:
		message = commandMessage(m.coreRedo())
	case operationCoreBack:
		if out, _, ok := m.outlinerPrimary(); ok {
			RunEscCascade(out.escSteps())
		}
	case operationCoreReset:
		message = JumpDashboardMsg{}
	case operationCoreHelp:
		message = ShowHelpMsg{
			SourceView: m.currentView(),
			ViewName:   "Outliner",
			Items:      m.coreInputHelpItems(),
		}
	case operationCoreQuit:
		message = QuitMsg{}
	case operationCoreNextWorkMode:
		if out, _, ok := m.outlinerPrimary(); ok {
			_, _ = out.cycleWorkMode(1)
		}
	case operationCorePreviousWorkMode:
		if out, _, ok := m.outlinerPrimary(); ok {
			_, _ = out.cycleWorkMode(-1)
		}
	default:
	}
	return OperationHandlerResult{Message: message}
}

func commandMessage(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return OperationCommandResultMsg{Cmd: cmd}
}

// coreCaptureMessage routes base capture to the workspace-owned System Inbox
// and project-focus capture to that focused project. Cursor project identity is
// deliberately ignored at base scope.
func (m *AppModel) coreCaptureMessage() tea.Msg {
	out, _, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return m.captureComposerMessage(nil, nil)
	}
	var project *models.Project
	if out.project != nil {
		project = out.project
	} else if out.isProjectScoped() {
		project = out.scopedProject()
	}
	if project == nil {
		// Resolve through RuntimeState's canonical workspace-owned Inbox
		// reference, then prefer the shell's loaded project pointer when it is
		// available so capture never borrows the base cursor's project.
		var inbox *models.Project
		if m.runtime != nil {
			inbox = m.runtime.SystemInboxProject()
			if inbox != nil {
				if loaded := m.projectByID(inbox.ID); loaded != nil {
					inbox = loaded
				}
			} else if id := m.runtime.Catalog().SystemInboxProjectID; id != "" {
				inbox = m.projectByID(id)
			}
		}
		return m.captureComposerMessage(inbox, m.activeProjects())
	}
	return m.captureComposerMessage(project, m.activeProjects())
}

func (m *AppModel) coreTransitionTask(target models.WorkflowState) tea.Cmd {
	out, _, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return nil
	}
	task, project := out.getTaskRef()
	if task == nil || project == nil {
		return nil
	}
	if out.taskItem.task != nil {
		return out.taskItem.Transition(taskIdentityFor(task, project), target)
	}
	// Canonical mode rows are filtered tree projections and may carry cloned
	// task nodes. Isolated models without a coordinator must mutate the owning
	// project's live task, never the projection that the next rebuild discards.
	if live := project.GetTaskByID(task.ID); live != nil {
		task = live
	}
	triagingInbox := m.activeInboxTriage() && task.State() == models.StateInbox
	if err := task.ApplyStateTransitionAt(target, time.Now()); err != nil {
		out.setStatus(err.Error())
		return nil
	}
	if m.runtime != nil {
		app.ReconcilePlanningState(m.runtime.Planning(), app.PlanningReconcileRequest{
			Targets: []app.PlanningReconcileTarget{{
				TaskID: task.ID, ProjectID: project.ID, State: task.State(),
			}},
		})
		m.rebuildPlanningFromRuntime()
	}
	if triagingInbox && task.State() != models.StateInbox {
		m.markInboxTriaged()
	}
	out.rebuildRows()
	return nil
}

func (m *AppModel) coreMoveToBacklog() tea.Cmd {
	task, project := m.outlinerTarget()
	if task == nil || project == nil {
		return nil
	}
	fresh, _, err := m.applyTaskPlanningAction(task.ID, project.ID, app.PlanningBacklog)
	if err != nil {
		m.err = err
		return errorCmd("%s", err)
	}
	m.setInfoStatus(fmt.Sprintf("Moved to Backlog: %s", taskTitleOrFallback(fresh, task)))
	return m.queueToggleResultCmd()
}

func (m *AppModel) coreMoveCursor(delta int) tea.Cmd {
	out, _, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return nil
	}
	key := "j"
	if delta < 0 {
		key = "k"
	}
	previous := out.cursor
	handled, cmd := handleListNavigation(
		key,
		&out.cursor,
		len(out.rows),
		out.contentHeight(),
		&out.cursorFlash,
	)
	if !handled {
		return nil
	}
	_, cmd = out.crossSkipNonNavigableRows(previous, cmd)
	return cmd
}

func (m *AppModel) coreReorder(delta int) tea.Cmd {
	out, _, ok := m.outlinerPrimary()
	if !ok || out == nil || !out.simplifiedReorderEligible() {
		if out != nil {
			out.orderingUnavailableStatus()
		}
		return nil
	}
	if out.isPlanningReorderMode() {
		_, cmd := out.planningReorderMove(delta)
		return cmd
	}
	return out.reorderTask(delta)
}

func (m *AppModel) coreTaskDetailMessage() tea.Msg {
	out, _, ok := m.outlinerPrimary()
	if !ok || out == nil || out.multiSelectMode {
		return nil
	}
	task, project := out.getTaskRef()
	if task == nil || project == nil {
		return nil
	}
	return ShowTaskDetailMsg{Task: task, Project: project}
}

func (m *AppModel) coreNotesMessage() tea.Msg {
	out, _, ok := m.outlinerPrimary()
	if ok && out != nil {
		if out.project != nil {
			return ShowNotesMsg{Project: out.project}
		}
		if project := out.scopedProject(); project != nil {
			return ShowNotesMsg{Project: project}
		}
	}
	return ShowGlobalNotesMsg{}
}

func (m *AppModel) coreTagsMessage(scopeProjectIDs []string) tea.Msg {
	out, _, ok := m.outlinerPrimary()
	if ok && out != nil {
		if out.project != nil {
			return ShowTagManagerMsg{Project: out.project}
		}
		if project := out.scopedProject(); project != nil {
			return ShowTagManagerMsg{Project: project}
		}
	}
	return ShowGlobalTagManagerMsg{ProjectSet: ActiveProjectSetScope(scopeProjectIDs)}
}

func (m *AppModel) coreUndo() tea.Cmd {
	if m.statusUndoCb != nil && time.Now().Before(m.statusExpiresAt) {
		callback := m.statusUndoCb
		m.clearStatus()
		return callback()
	}
	if cmd := m.handleUndoToggle(); cmd != nil {
		return cmd
	}
	_, project := m.getCurrentCursorTaskAndProject()
	return m.performGlobalTaskUndoRedoForProject(project, false)
}

func (m *AppModel) coreRedo() tea.Cmd {
	if cmd := m.handleRedoToggle(); cmd != nil {
		return cmd
	}
	_, project := m.getCurrentCursorTaskAndProject()
	return m.performGlobalTaskUndoRedoForProject(project, true)
}

func (m *AppModel) coreInputHelpItems() []HelpItem {
	if m.operationRegistry == nil {
		return nil
	}
	return m.operationRegistry.HelpItemsForInputStrategy(
		InputStrategySimplified,
		m.operationCommandContext(),
	)
}
