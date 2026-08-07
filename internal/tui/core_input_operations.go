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
		operationCorePeek,
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
		core(operationCoreBacklog, "Move to Backlog", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "backlog"),
		core(operationCoreToday, "Toggle Today", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "today"),
		core(operationCoreNext, "Toggle Next", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskMutation, "next"),
		core(operationCoreActions, "Task Actions", OperationCategoryTask, TargetCardinalityOneOrMore, OperationRiskReadOnly, "action sheet"),
		core(operationCoreCursorDown, "Move Cursor Down", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorUp, "Move Cursor Up", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorTop, "Go to First Row", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorBottom, "Go to Last Row", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorPageUp, "Half Page Up", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCursorPageDown, "Half Page Down", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreJumpBackward, "Jump to Older Task Position", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreJumpForward, "Jump to Newer Task Position", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreCollapseParent, "Collapse or Go to Parent", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreExpand, "Expand", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreToggleSections, "Toggle All Sections", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreReorderDown, "Reorder Down", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(operationCoreReorderUp, "Reorder Up", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		peek,
		core(operationCoreScopeSearch, "Search Scoped Tasks", OperationCategoryFilter, TargetCardinalityNone, OperationRiskReadOnly, "scope-wide search"),
		core(operationCoreSelectProject, "Select Project", OperationCategoryProject, TargetCardinalityNone, OperationRiskReadOnly, "projects", "switch project"),
		core(operationCoreMultiSelect, "Toggle Task Selection", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "visual selection"),
		core(operationCoreRangeSelect, "Select Task Range", OperationCategoryTask, TargetCardinalityNone, OperationRiskReadOnly, "contiguous selection", "range selection"),
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
		switch descriptor.ID { //nolint:exhaustive // Only outliner navigation operations override applicability.
		case operationCoreNextWorkMode,
			operationCorePreviousWorkMode,
			operationCoreCollapseParent,
			operationCoreExpand,
			operationCoreToggleSections,
			operationCoreReorderDown,
			operationCoreReorderUp:
			descriptor.Applicability = func(ctx CommandContext) bool {
				return coreOutlinerApplicability(ctx) &&
					(m.crossOutliner == nil || !m.crossOutliner.hasSearchProjection())
			}
		}
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

// coreInputOperationRequiresTask names the delegated core operations whose
// handler branch does nothing at all without a task. Delegated bindings bypass
// registry target resolution, so an unmet requirement reaches the user as a
// silent no-op unless it is refused here.
func coreInputOperationRequiresTask(id OperationID) bool {
	switch id { //nolint:exhaustive // Only the task-consuming core operations are listed.
	case operationCoreEdit,
		operationCoreEditDescription,
		operationCoreExternalEditor,
		operationCoreBacklog,
		operationCoreToday,
		operationCoreNext:
		return true
	default:
		return false
	}
}

//nolint:gocyclo // The direct-key dispatcher keeps the final key grammar in one auditable switch.
func (m *AppModel) executeCoreInputOperation(invocation OperationInvocation) OperationHandlerResult {
	if coreInputOperationRequiresTask(invocation.Descriptor.ID) &&
		!invocation.Context.Selection.Visual &&
		!currentTaskIdentity(invocation.Context).Valid() {
		return OperationHandlerResult{Err: operationTargetRejection(invocation.Descriptor)}
	}

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
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		if invocation.Context.Selection.Visual {
			message = commandMessage(m.corePlanningOperation(invocation.Targets, app.PlanningBacklog, capture))
		} else {
			message = commandMessage(m.coreMoveToBacklogSingle(capture))
		}
	case operationCoreToday:
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		if invocation.Context.Selection.Visual {
			message = commandMessage(m.coreToggleToday(invocation.Targets, capture))
		} else {
			message = commandMessage(m.toggleToday(capture))
		}
	case operationCoreNext:
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		if invocation.Context.Selection.Visual {
			message = commandMessage(m.coreToggleNext(invocation.Targets, capture))
		} else {
			message = commandMessage(m.toggleQueue(capture))
		}
	case operationCoreActions:
		if actionErr := m.openTaskActionSheet(invocation.Context); actionErr != nil {
			m.setInfoStatus(actionErr.Error())
			message = commandMessage(defaultStatusTimeout())
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
	case operationCoreJumpBackward:
		if out, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.jumpBackward())
		}
	case operationCoreJumpForward:
		if out, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.jumpForward())
		}
	case operationCoreCollapseParent:
		if out, ok := m.outlinerPrimary(); ok {
			out.collapseOrMoveToParent()
		}
	case operationCoreExpand:
		if out, ok := m.outlinerPrimary(); ok {
			out.expandAtCursor()
		}
	case operationCoreToggleSections:
		if out, ok := m.outlinerPrimary(); ok {
			out.toggleVisibleSections()
		}
	case operationCoreReorderDown:
		message = commandMessage(m.coreReorder(1))
	case operationCoreReorderUp:
		message = commandMessage(m.coreReorder(-1))
	case operationCorePeek:
		if out, ok := m.outlinerPrimary(); ok {
			out.togglePeek()
		}
	case operationCoreScopeSearch:
		if out, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.ActivateSearch())
		}
	case operationCoreSelectProject:
		message = ShowProjectHubMsg{}
	case operationCoreMultiSelect:
		if out, ok := m.outlinerPrimary(); ok {
			out.toggleCursorSelection()
		}
	case operationCoreRangeSelect:
		if out, ok := m.outlinerPrimary(); ok {
			out.startCursorRangeSelection()
		}
	case operationCoreSelectAll:
		if out, ok := m.outlinerPrimary(); ok {
			out.selectAllVisible()
			out.updateVisualSelectionStatus()
		}
	case operationCoreNotes:
		message = ShowNotesManagerMsg{Scope: AllNotesScope()}
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
		if out, ok := m.outlinerPrimary(); ok {
			RunEscCascade(out.escSteps())
			// Leaving a parked focus drops the project from the snapshot in place;
			// the cascade cannot return a Cmd, so the reload is claimed here.
			message = commandMessage(out.takeParkedFocusReload())
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
		if out, ok := m.outlinerPrimary(); ok {
			_, _ = out.cycleWorkMode(1)
		}
	case operationCorePreviousWorkMode:
		if out, ok := m.outlinerPrimary(); ok {
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
	out, ok := m.outlinerPrimary()
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
		// Resolve through RuntimeState's canonical workspace-owned Unfiled
		// reference, then prefer the shell's loaded project pointer when it is
		// available so capture never borrows the base cursor's project.
		var unfiled *models.Project
		if m.runtime != nil {
			unfiled = m.runtime.SystemUnfiledProject()
			if unfiled != nil {
				if loaded := m.projectByID(unfiled.ID); loaded != nil {
					unfiled = loaded
				}
			} else if id := m.runtime.Catalog().SystemUnfiledProjectID; id != "" {
				unfiled = m.projectByID(id)
			}
		}
		return m.captureComposerMessage(unfiled, m.availableProjects())
	}
	return m.captureComposerMessage(project, m.availableProjects())
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

func planningMemberLocators(members map[TaskIdentity]bool) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(members))
	for identity, member := range members {
		if member {
			locators = append(locators, identity.Locator())
		}
	}
	return locators
}

func (m *AppModel) corePlanningOperation(
	targets ResolvedOperationTargets,
	action app.PlanningAction,
	capture *outlinerRelocationCapture,
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
	return withOutlinerRelocationCapture(
		m.secondaryFeatures.Bulk.ApplyPlanningLocators(locators, action, summary),
		capture,
	)
}

func (m *AppModel) coreMoveToBacklogSingle(capture *outlinerRelocationCapture) tea.Cmd {
	task, project := m.outlinerTarget()
	if task == nil || project == nil {
		return nil
	}
	fresh, _, transactionID, err := m.applyTaskPlanningAction(task.ID, project.ID, app.PlanningBacklog)
	if err != nil {
		m.err = err
		return errorCmd("%s", err)
	}
	successMessage := fmt.Sprintf("Moved to Backlog: %s", taskTitleOrFallback(fresh, task))
	return m.finishSynchronousOutlinerRelocation(m.queueToggleResultCmd(), capture, successMessage, transactionID)
}

func (m *AppModel) coreToggleToday(targets ResolvedOperationTargets, capture *outlinerRelocationCapture) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	action, err := app.ResolvePlanningToggleAction(
		app.PlanningHorizonToday,
		locators,
		planningMemberLocators(m.todayIDs),
	)
	if err != nil {
		return errorCmd("%s", err)
	}
	return m.corePlanningOperation(targets, action, capture)
}

func (m *AppModel) coreToggleNext(targets ResolvedOperationTargets, capture *outlinerRelocationCapture) tea.Cmd {
	locators := operationTargetLocators(targets)
	if len(locators) == 0 {
		return nil
	}
	action, err := app.ResolvePlanningToggleAction(
		app.PlanningHorizonNext,
		locators,
		planningMemberLocators(m.nextOrderIDs),
	)
	if err != nil {
		return errorCmd("%s", err)
	}
	return m.corePlanningOperation(targets, action, capture)
}

// coreMoveCursor takes a shared list-navigation key rather than a delta so steps,
// edge jumps, and half-page jumps all inherit the same viewport sizing, boundary
// flashing, and non-navigable row skipping.
func (m *AppModel) coreMoveCursor(key string) tea.Cmd {
	out, ok := m.outlinerPrimary()
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
	out.updateRangeSelection()
	return cmd
}

func (m *AppModel) coreReorder(delta int) tea.Cmd {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil || !out.simplifiedReorderEligible() {
		if out != nil {
			out.orderingUnavailableStatus()
		}
		return nil
	}
	if out.isPinnedReorderRow() {
		return out.pinnedReorderMove(delta)
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
