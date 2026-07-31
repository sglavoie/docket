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
	// Single-task operations must not inherit a visual selection or a project
	// default when the cursor does not identify one task.
	withTaskTarget := func(descriptor OperationDescriptor) OperationDescriptor {
		descriptor.TargetSource = OperationTargetCurrentTask
		descriptor.Applicability = func(ctx CommandContext) bool {
			return coreOutlinerApplicability(ctx) && !ctx.Selection.Visual
		}
		return descriptor
	}
	peek := core(
		operationCoreDetail,
		"Toggle Task Peek",
		OperationCategoryNavigation,
		TargetCardinalityNone,
		OperationRiskReadOnly,
		"peek task",
		"task preview",
	)
	peek.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) && !ctx.Selection.Visual
	}
	return []OperationDescriptor{
		core(operationCoreCapture, "Capture", OperationCategoryTask, TargetCardinalityNone, OperationRiskMutation, "add task", "inbox capture"),
		withTaskTarget(core(operationCoreEdit, "Edit Title", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "rename task", "change title")),
		withTaskTarget(core(operationCoreEditDescription, "Edit Description", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "change description", "edit body")),
		withTaskTarget(core(operationCoreExternalEditor, "Open in External Editor", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "edit task externally", "external editor")),
		core(operationCoreBacklog, "Move to Backlog", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "ready"),
		core(operationCoreToday, "Toggle Today", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "daily plan"),
		core(operationCoreNext, "Toggle Next", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "on deck"),
		core(operationCoreActions, "Task Actions", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskReadOnly, "action sheet"),
		core(operationCoreCursorDown, "Move Cursor Down", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorUp, "Move Cursor Up", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorTop, "Go to First Row", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorBottom, "Go to Last Row", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorPageUp, "Half Page Up", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorPageDown, "Half Page Down", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCollapseParent, "Collapse or Go to Parent", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreExpand, "Expand", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreToggleSections, "Toggle All Sections", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreReorderDown, "Reorder Down", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(operationCoreReorderUp, "Reorder Up", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		peek,
		core(operationCoreVisibleSearch, "Search Visible Tasks", OperationCategoryFilter, TargetCardinalityNone, OperationRiskReadOnly, "filter visible list"),
		core(operationCoreScope, "Choose Scope", OperationCategoryScope, TargetCardinalityNone, OperationRiskReadOnly, "contexts"),
		core(operationCoreSelectProject, "Select Project", OperationCategoryProject, TargetCardinalityNone, OperationRiskReadOnly, "projects", "switch project"),
		core(operationCoreMultiSelect, "Toggle Task Selection", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "visual selection"),
		core(operationCoreSelectAll, "Select All Visible Tasks", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "select visible"),
		core(operationCoreNotes, "Notes", OperationCategoryNote, TargetCardinalityNone, OperationRiskReadOnly, "scratch notes"),
		core(operationCoreToggleTheme, "Toggle Theme", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation, "dark theme", "light theme"),
		core(operationCoreUndo, "Undo", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation),
		core(operationCoreRedo, "Redo", OperationCategorySystem, TargetCardinalityNone, OperationRiskMutation),
		core(operationCoreBack, "Back", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
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
			Applicability:            descriptor.Applicability,
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
	var err error
	//nolint:exhaustive // Only core input operations are registered with this handler.
	switch invocation.Descriptor.ID {
	case operationCoreCapture:
		message = m.coreCaptureMessage()
	case operationCoreEdit:
		if task, project := m.taskAndProjectForTargets(invocation.Targets); task != nil {
			message = ShowTaskTextEditorMsg{
				Task: task, Project: project, Operation: app.TaskOperationEditTitle,
			}
		}
	case operationCoreEditDescription:
		if task, project := m.taskAndProjectForTargets(invocation.Targets); task != nil {
			message = ShowTaskTextEditorMsg{
				Task: task, Project: project, Operation: app.TaskOperationEditDescription,
			}
		}
	case operationCoreExternalEditor:
		if task, project := m.taskAndProjectForTargets(invocation.Targets); task != nil {
			message = EditTaskContentMsg{Task: task, Project: project}
		}
	case operationCoreBacklog:
		if invocation.Context.Selection.Visual {
			message = commandMessage(m.corePlanningOperation(invocation.Targets, app.PlanningBacklog))
		} else {
			message = commandMessage(m.coreMoveToBacklogSingle())
		}
	case operationCoreToday:
		if invocation.Context.Selection.Visual {
			message = commandMessage(m.coreToggleToday(invocation.Targets))
		} else {
			message = commandMessage(m.toggleToday())
		}
	case operationCoreNext:
		if invocation.Context.Selection.Visual {
			message = commandMessage(m.coreToggleNext(invocation.Targets))
		} else {
			message = commandMessage(m.toggleQueue())
		}
	case operationCoreActions:
		if actionErr := m.openTaskActionSheet(invocation.Context); actionErr != nil {
			m.setInfoStatus(actionErr.Error())
		}
	case operationCoreCursorDown:
		message = commandMessage(m.coreMoveCursor("j"))
	case operationCoreCursorUp:
		message = commandMessage(m.coreMoveCursor("k"))
	case operationCoreCursorTop:
		message = commandMessage(m.coreMoveCursor("home"))
	case operationCoreCursorBottom:
		message = commandMessage(m.coreMoveCursor("G"))
	case operationCoreCursorPageUp:
		message = commandMessage(m.coreMoveCursor("ctrl+u"))
	case operationCoreCursorPageDown:
		message = commandMessage(m.coreMoveCursor("ctrl+d"))
	case operationCoreCollapseParent:
		if out, _, ok := m.outlinerPrimary(); ok {
			out.collapseOrMoveToParent()
		}
	case operationCoreExpand:
		if out, _, ok := m.outlinerPrimary(); ok {
			out.expandAtCursor()
		}
	case operationCoreToggleSections:
		if out, _, ok := m.outlinerPrimary(); ok {
			out.toggleVisibleSections()
		}
	case operationCoreReorderDown:
		message = commandMessage(m.coreReorder(1))
	case operationCoreReorderUp:
		message = commandMessage(m.coreReorder(-1))
	case operationCoreDetail:
		if out, _, ok := m.outlinerPrimary(); ok {
			out.togglePeek()
		}
	case operationCoreVisibleSearch:
		if out, _, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.ActivateSearch())
		}
	case operationCoreScope:
		message = ShowContextHubMsg{}
	case operationCoreSelectProject:
		message = ShowProjectHubMsg{}
	case operationCoreMultiSelect:
		if out, _, ok := m.outlinerPrimary(); ok {
			out.toggleCursorSelection()
		}
	case operationCoreSelectAll:
		if out, _, ok := m.outlinerPrimary(); ok {
			out.peekMode = false
			out.multiSelectMode = true
			out.selectAllVisible()
			out.updateVisualSelectionStatus()
		}
	case operationCoreNotes:
		project := m.projectByID(invocation.Context.FocusedProjectID)
		if project != nil {
			message = ShowNotesMsg{Project: project}
		} else {
			message = ShowGlobalNotesMsg{}
		}
	case operationCoreToggleTheme:
		err = m.coreToggleTheme()
		if err == nil {
			message = commandMessage(defaultStatusTimeout())
		}
	case operationCoreUndo:
		message = commandMessage(m.coreUndo())
	case operationCoreRedo:
		message = commandMessage(m.coreRedo())
	case operationCoreBack:
		if out, _, ok := m.outlinerPrimary(); ok {
			RunEscCascade(out.escSteps())
		}
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
	return OperationHandlerResult{Message: message, Err: err}
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

func operationTargetLocators(targets ResolvedOperationTargets) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if target.Task.Valid() {
			locators = append(locators, target.Task.Locator())
		}
	}
	return locators
}

func (m *AppModel) corePlanningOperation(
	targets ResolvedOperationTargets,
	action app.PlanningAction,
) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	summary := fmt.Sprintf("Updated %d task(s)", len(locators))
	switch action {
	case app.PlanningAddToday:
		summary = fmt.Sprintf("Added %d task(s) to Today", len(locators))
	case app.PlanningRemoveToday:
		summary = fmt.Sprintf("Removed %d task(s) from Today", len(locators))
	case app.PlanningAddNext:
		summary = fmt.Sprintf("Moved %d task(s) to Next", len(locators))
	case app.PlanningRemoveNext:
		summary = fmt.Sprintf("Removed %d task(s) from Next", len(locators))
	case app.PlanningBacklog:
		summary = fmt.Sprintf("Moved %d task(s) to Backlog", len(locators))
	}
	return m.secondaryFeatures.Bulk.ApplyPlanningLocators(locators, action, summary)
}

func (m *AppModel) coreMoveToBacklogSingle() tea.Cmd {
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

func (m *AppModel) coreToggleToday(targets ResolvedOperationTargets) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	allToday := true
	for _, locator := range locators {
		if !m.todayIDs[taskIdentityFromLocator(locator)] {
			allToday = false
			break
		}
	}
	action := app.PlanningAddToday
	if allToday {
		action = app.PlanningRemoveToday
	}
	return m.corePlanningOperation(targets, action)
}

func (m *AppModel) coreToggleNext(targets ResolvedOperationTargets) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	allNext := true
	for _, locator := range locators {
		task, _ := m.taskAndProjectForIdentity(taskIdentityFromLocator(locator))
		if !taskIsNext(task) {
			allNext = false
			break
		}
	}
	action := app.PlanningAddNext
	if allNext {
		action = app.PlanningRemoveNext
	}
	return m.corePlanningOperation(targets, action)
}

// coreMoveCursor takes a shared list-navigation key rather than a delta so steps,
// edge jumps, and half-page jumps all inherit the same viewport sizing, boundary
// flashing, and non-navigable row skipping.
func (m *AppModel) coreMoveCursor(key string) tea.Cmd {
	out, _, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return nil
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

// coreToggleTheme flips exactly between the two maintained machine-local
// variants and rebuilds every active style immediately.
func (m *AppModel) coreToggleTheme() error {
	if m.runtime == nil || m.runtime.Machine() == nil {
		return fmt.Errorf("machine theme is unavailable")
	}
	machine := *m.runtime.Machine()
	if machine.Theme == "light" {
		machine.Theme = "dark"
	} else {
		machine.Theme = "light"
	}
	if err := m.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save theme: %w", err)
	}
	ApplyTheme(NewThemeConfig(m.runtime.Machine()))
	m.setInfoStatus("Theme: " + machine.Theme)
	return nil
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
