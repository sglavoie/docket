package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// Stable core operation IDs let the final grammar remain independent of dormant
// legacy command adapters and their retired shortcuts or prefix chords.
const (
	operationCoreCapture            OperationID = "tui.core.capture"
	operationCoreEdit               OperationID = "tui.core.edit"
	operationCoreEditDescription    OperationID = "tui.core.edit-description"
	operationCoreExternalEditor     OperationID = "tui.core.external-editor"
	operationCoreOutlineEditor      OperationID = "tui.core.outline-editor"
	operationCoreBacklog            OperationID = "tui.core.backlog"
	operationCoreToday              OperationID = "tui.core.today"
	operationCoreNext               OperationID = "tui.core.next"
	operationCoreActions            OperationID = "tui.core.actions"
	operationCoreCursorDown         OperationID = "tui.core.cursor-down"
	operationCoreCursorUp           OperationID = "tui.core.cursor-up"
	operationCoreCursorTop          OperationID = "tui.core.cursor-top"
	operationCoreCursorBottom       OperationID = "tui.core.cursor-bottom"
	operationCoreCursorPageUp       OperationID = "tui.core.cursor-page-up"
	operationCoreCursorPageDown     OperationID = "tui.core.cursor-page-down"
	operationCoreCursorScreenTop    OperationID = "tui.core.cursor-screen-top"
	operationCoreCursorScreenMiddle OperationID = "tui.core.cursor-screen-middle"
	operationCoreCursorScreenBottom OperationID = "tui.core.cursor-screen-bottom"
	operationCoreScrollCursorTop    OperationID = "tui.core.scroll-cursor-top"
	operationCoreScrollCursorMiddle OperationID = "tui.core.scroll-cursor-middle"
	operationCoreScrollCursorBottom OperationID = "tui.core.scroll-cursor-bottom"
	operationCoreNextHeader         OperationID = "tui.core.next-header"
	operationCorePreviousHeader     OperationID = "tui.core.previous-header"
	operationCoreJumpBackward       OperationID = "tui.core.jump-backward"
	operationCoreJumpForward        OperationID = "tui.core.jump-forward"
	operationCoreFocusLatest        OperationID = "tui.core.focus-latest"
	operationCoreGotoTaskProject    OperationID = "tui.core.goto-task-project"
	operationCoreCollapseParent     OperationID = "tui.core.collapse-parent"
	operationCoreExpand             OperationID = "tui.core.expand"
	operationCoreToggleSections     OperationID = "tui.core.toggle-sections"
	operationCoreToggleProjectGroup OperationID = "tui.core.toggle-project-groups"
	operationCoreToggleSubtrees     OperationID = "tui.core.toggle-subtrees"
	operationCoreReorderDown        OperationID = "tui.core.reorder-down"
	operationCoreReorderUp          OperationID = "tui.core.reorder-up"
	operationCoreReorderBottom      OperationID = "tui.core.reorder-bottom"
	operationCoreReorderTop         OperationID = "tui.core.reorder-top"
	operationCoreFocusSubtree       OperationID = "tui.core.focus-subtree"
	operationCoreFocusParent        OperationID = "tui.core.focus-parent"
	operationCoreActivateGroup      OperationID = "tui.core.activate-group"
	operationCoreZoomSection        OperationID = "tui.core.zoom-section"
	operationCorePeek               OperationID = "tui.core.peek"
	operationCoreScopeSearch        OperationID = "tui.core.scope-search"
	operationCoreSelectProject      OperationID = "tui.core.select-project"
	operationCoreMultiSelect        OperationID = "tui.core.multi-select"
	operationCoreRangeSelect        OperationID = "tui.core.range-select"
	operationCoreSelectAll          OperationID = "tui.core.select-all"
	operationCoreNotes              OperationID = "tui.core.notes"
	operationCoreToggleTheme        OperationID = "tui.core.toggle-theme"
	operationCoreUndo               OperationID = "tui.core.undo"
	operationCoreRedo               OperationID = "tui.core.redo"
	operationCoreBack               OperationID = "tui.core.back"
	operationCoreHelp               OperationID = "tui.core.help"
	operationCoreQuit               OperationID = "tui.core.quit"
	operationCoreNextWorkMode       OperationID = "tui.core.next-work-mode"
	operationCorePreviousWorkMode   OperationID = "tui.core.previous-work-mode"
	operationCoreGotoAll            OperationID = "tui.core.goto-all"
	operationCoreGotoInbox          OperationID = "tui.core.goto-inbox"
	operationCoreGotoToday          OperationID = "tui.core.goto-today"
	operationCoreGotoNext           OperationID = "tui.core.goto-next"
	operationCoreGotoBacklog        OperationID = "tui.core.goto-backlog"
	operationCoreGotoDue            OperationID = "tui.core.goto-due"
	operationCoreGotoRecent         OperationID = "tui.core.goto-recent"
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
	peek := withTaskTarget(core(
		operationCorePeek,
		"Toggle Task Peek",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"peek task",
		"task preview",
	))
	peek.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			currentTaskIdentity(ctx).Valid()
	}
	focusSubtree := withTaskTarget(core(
		operationCoreFocusSubtree,
		"Focus Subtree",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"focus task",
		"focus descendants",
	))
	focusSubtree.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			currentTaskIdentity(ctx).Valid()
	}
	focusParent := withTaskTarget(core(
		operationCoreFocusParent,
		"Focus Parent",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"focus parent",
		"widen focus",
	))
	focusParent.Applicability = focusSubtree.Applicability
	gotoTaskProject := withTaskTarget(core(
		operationCoreGotoTaskProject,
		"Go to Task's Project",
		OperationCategoryNavigation,
		TargetCardinalityOne,
		OperationRiskReadOnly,
		"open task project",
		"scope to task project",
	))
	gotoTaskProject.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			currentTaskIdentity(ctx).Valid()
	}
	zoomSection := core(
		operationCoreZoomSection,
		"Zoom to Section",
		OperationCategoryNavigation,
		TargetCardinalityNone,
		OperationRiskReadOnly,
		"zoom section",
		"section only",
		"narrow to section",
	)
	zoomSection.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			ctx.Entity.HasSectionHeaderCursor
	}
	activateGroup := core(
		operationCoreActivateGroup,
		"Open Project",
		OperationCategoryNavigation,
		TargetCardinalityNone,
		OperationRiskReadOnly,
		"focus project",
		"scope to project",
	)
	activateGroup.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) &&
			!ctx.Selection.Visual &&
			!ctx.Entity.HasChecklistCursor &&
			!ctx.Entity.HasSectionHeaderCursor &&
			!currentTaskIdentity(ctx).Valid()
	}
	// The outline editor restructures a whole project tree, so it names no task
	// target and refuses a visual selection: the buffer's scope is the project the
	// cursor sits in, and a selection would claim a narrower one it cannot honor.
	outlineEditor := core(
		operationCoreOutlineEditor,
		"Outline Edit Project",
		OperationCategoryProject,
		TargetCardinalityNone,
		OperationRiskMutation,
		"bulk restructure",
		"outline buffer",
	)
	outlineEditor.Applicability = func(ctx CommandContext) bool {
		return coreOutlinerApplicability(ctx) && !ctx.Selection.Visual
	}
	descriptors := []OperationDescriptor{
		core(operationCoreCapture, "Capture", OperationCategoryTask, TargetCardinalityNone, OperationRiskMutation, "add task", "inbox capture"),
		withTaskTarget(core(operationCoreEdit, "Edit Task", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "edit task line", "rename task", "change title")),
		withTaskTarget(core(operationCoreEditDescription, "Edit Description", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "change description", "edit body")),
		withTaskTarget(core(operationCoreExternalEditor, "Open in External Editor", OperationCategoryTask, TargetCardinalityOne, OperationRiskMutation, "edit task externally", "external editor")),
		outlineEditor,
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
		core(operationCoreCursorScreenTop, "Go to Top of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "screen top", "highest visible row"),
		core(operationCoreCursorScreenMiddle, "Go to Middle of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "screen middle", "middle visible row"),
		core(operationCoreCursorScreenBottom, "Go to Bottom of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "screen bottom", "lowest visible row"),
		core(operationCoreScrollCursorTop, "Scroll Cursor to Top of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "scroll top", "row to top"),
		core(operationCoreScrollCursorMiddle, "Scroll Cursor to Middle of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "scroll middle", "row to middle"),
		core(operationCoreScrollCursorBottom, "Scroll Cursor to Bottom of Screen", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "scroll bottom", "row to bottom"),
		core(operationCoreNextHeader, "Next Section", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "jump to next section", "next group"),
		core(operationCorePreviousHeader, "Previous Section", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "jump to previous section", "previous group"),
		core(operationCoreJumpBackward, "Jump to Older Task Position", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreJumpForward, "Jump to Newer Task Position", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreFocusLatest, "Focus Latest Activity", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "latest activity", "last touched"),
		core(operationCoreCollapseParent, "Collapse or Go to Parent", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		core(operationCoreExpand, "Expand", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly),
		// The three fold axes are three toggles, not six directional operations:
		// each owns one map, and the aliases stay direction-free because a toggle
		// must not advertise a way it does not always go.
		core(operationCoreToggleSections, "Toggle All Sections", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "fold sections"),
		core(operationCoreToggleProjectGroup, "Toggle All Project Groups", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "fold project groups"),
		core(operationCoreToggleSubtrees, "Toggle All Subtrees", OperationCategoryNavigation, TargetCardinalityNone, OperationRiskReadOnly, "fold subtrees"),
		core(operationCoreReorderDown, "Reorder Down", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(operationCoreReorderUp, "Reorder Up", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(operationCoreReorderBottom, "Reorder to Bottom", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		core(operationCoreReorderTop, "Reorder to Top", OperationCategoryNavigation, TargetCardinalityOne, OperationRiskMutation),
		focusSubtree,
		focusParent,
		gotoTaskProject,
		zoomSection,
		activateGroup,
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
		core(operationCoreGotoAll, "Go to All", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "all mode"),
		core(operationCoreGotoInbox, "Go to Inbox", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "inbox mode"),
		core(operationCoreGotoToday, "Go to Today", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "today mode"),
		core(operationCoreGotoNext, "Go to Next", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "next mode"),
		core(operationCoreGotoBacklog, "Go to Backlog", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "backlog mode"),
		core(operationCoreGotoDue, "Go to Due", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "due mode"),
		core(operationCoreGotoRecent, "Go to Recent", OperationCategoryMode, TargetCardinalityNone, OperationRiskReadOnly, "recent mode", "recent activity"),
	}
	// A cursor that positively names a project belongs to the project grammar, so
	// a core operation that consumes a task must not bind there. Without this the
	// s chord would still pend on a project header, because its three planning
	// children stayed applicable and kept the whole subtree alive.
	for i := range descriptors {
		if !coreInputOperationRequiresTask(descriptors[i].ID) {
			continue
		}
		inner := descriptors[i].Applicability
		descriptors[i].Applicability = func(ctx CommandContext) bool {
			return inner(ctx) && (ctx.Selection.Visual || !commandContextNamesNonTaskEntity(ctx))
		}
	}
	return descriptors
}

// coreInputOperationRefusesSearchProjection names the outliner navigation
// operations whose projection the temporary search replacement owns, so they
// refuse instead of reshaping a projection the user has not exited. The two
// Focus jumps and Go to Task's Project are deliberately absent: leaving the
// projection is what they are for, so they exit search onto the result instead.
func coreInputOperationRefusesSearchProjection(id OperationID) bool {
	if _, ok := gotoModeOperations[id]; ok {
		return true
	}
	switch id { //nolint:exhaustive // Only outliner navigation operations override applicability.
	case operationCoreNextWorkMode,
		operationCorePreviousWorkMode,
		operationCoreFocusLatest,
		operationCoreCollapseParent,
		operationCoreExpand,
		operationCoreToggleSections,
		operationCoreToggleProjectGroup,
		operationCoreToggleSubtrees,
		operationCoreReorderDown,
		operationCoreReorderUp,
		operationCoreReorderBottom,
		operationCoreReorderTop:
		return true
	default:
		return false
	}
}

// registerCoreInputOperations stages native final-grammar operations without
// adding them to the retained legacy Command palette.
func (m *AppModel) registerCoreInputOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	for _, descriptor := range coreInputOperationDescriptors() {
		if coreInputOperationRefusesSearchProjection(descriptor.ID) {
			inner := descriptor.Applicability
			descriptor.Applicability = func(ctx CommandContext) bool {
				return (inner == nil || inner(ctx)) &&
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
		operationCoreNext,
		operationCoreFocusSubtree,
		operationCoreFocusParent,
		operationCoreGotoTaskProject,
		operationCorePeek:
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
			message = StartTaskLineEditMsg{Task: task, Project: project}
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
	case operationCoreOutlineEditor:
		project, _, refusal := m.outlineEditorRequest()
		switch {
		case refusal != "":
			m.setWarningStatus(refusal)
			message = commandMessage(defaultStatusTimeout())
		case project == nil:
			m.setWarningStatus("No project under the cursor to outline")
			message = commandMessage(defaultStatusTimeout())
		default:
			message = OpenOutlineEditorMsg{Project: project}
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
	case operationCoreCursorScreenTop:
		return m.coreMoveCursorScreen(screenJumpTop)
	case operationCoreCursorScreenMiddle:
		return m.coreMoveCursorScreen(screenJumpMiddle)
	case operationCoreCursorScreenBottom:
		return m.coreMoveCursorScreen(screenJumpBottom)
	case operationCoreScrollCursorTop:
		return m.coreScrollCursorTo(viewScrollAnchorTop)
	case operationCoreScrollCursorMiddle:
		return m.coreScrollCursorTo(viewScrollAnchorMiddle)
	case operationCoreScrollCursorBottom:
		return m.coreScrollCursorTo(viewScrollAnchorBottom)
	case operationCoreNextHeader, operationCorePreviousHeader:
		delta := 1
		if invocation.Descriptor.ID == operationCorePreviousHeader {
			delta = -1
		}
		if out, ok := m.outlinerPrimary(); ok {
			cmd, moved := out.moveCursorToAdjacentHeader(delta)
			if !moved {
				return OperationHandlerResult{Status: "No sections to jump between"}
			}
			message = commandMessage(cmd)
		}
	case operationCoreJumpBackward:
		if out, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.jumpBackward())
		}
	case operationCoreJumpForward:
		if out, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.jumpForward())
		}
	case operationCoreFocusLatest:
		if out, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.focusLatestActivity())
		}
	case operationCoreGotoTaskProject:
		return m.coreGotoTaskProject(invocation.Targets)
	case operationCoreCollapseParent:
		if out, ok := m.outlinerPrimary(); ok {
			out.collapseOrMoveToParent()
		}
	case operationCoreExpand:
		if out, ok := m.outlinerPrimary(); ok {
			out.expandAtCursor()
		}
	// The three fold toggles report their refusals and nothing else: a productive
	// press returns a zero result, so it arms no footer flash for a change already
	// on screen. p names P because it always refuses inside a project scope, which
	// is exactly where the key used to open Select Project.
	case operationCoreToggleSections:
		if out, ok := m.outlinerPrimary(); ok {
			if !out.toggleVisibleSections() {
				return OperationHandlerResult{Status: "No foldable sections here"}
			}
		}
	case operationCoreToggleProjectGroup:
		if out, ok := m.outlinerPrimary(); ok {
			if !out.toggleVisibleProjectGroups() {
				return OperationHandlerResult{Status: "No project groups here; P switches project"}
			}
		}
	case operationCoreToggleSubtrees:
		if out, ok := m.outlinerPrimary(); ok {
			if !out.toggleAllSubtrees() {
				return OperationHandlerResult{Status: "No foldable subtrees here"}
			}
		}
	case operationCoreReorderDown:
		message = commandMessage(m.coreReorder(1))
	case operationCoreReorderUp:
		message = commandMessage(m.coreReorder(-1))
	case operationCoreReorderBottom:
		message = commandMessage(m.coreReorderEdge(1))
	case operationCoreReorderTop:
		message = commandMessage(m.coreReorderEdge(-1))
	case operationCoreFocusSubtree:
		if out, ok := m.outlinerPrimary(); ok {
			if status, landed := out.exitSearchOntoCursorTask(); !landed {
				return OperationHandlerResult{Status: status}
			}
			if !out.focusSubtreeAtCursor() {
				return OperationHandlerResult{Status: "No children to focus"}
			}
		}
	case operationCoreFocusParent:
		if out, ok := m.outlinerPrimary(); ok {
			if status, landed := out.exitSearchOntoCursorTask(); !landed {
				return OperationHandlerResult{Status: status}
			}
			if !out.focusParentAtCursor() {
				return OperationHandlerResult{Status: "No parent to focus"}
			}
		}
	case operationCoreZoomSection:
		if out, ok := m.outlinerPrimary(); ok {
			out.zoomSectionAtCursor()
		}
	case operationCoreActivateGroup:
		if out, ok := m.outlinerPrimary(); ok {
			out.activateGroupAtCursor()
		}
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
		message = ShowNotesManagerMsg{Scope: m.coreNotesScope()}
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
			ViewName: "Outliner",
			Items:    m.coreInputHelpItems(),
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
		if mode, ok := gotoModeOperations[invocation.Descriptor.ID]; ok {
			if out, outlinerOK := m.outlinerPrimary(); outlinerOK {
				_, _ = out.selectOutlinerMode(mode)
			}
		}
	}
	return OperationHandlerResult{Message: message, Err: err}
}

func commandMessage(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return OperationCommandResultMsg{Cmd: cmd}
}

// coreNotesScope keeps Ctrl+N on All Notes everywhere except a project header
// row, which names one project explicitly rather than incidentally — the same
// exception coreCaptureMessage makes, read through the same predicate, so no
// task cursor can narrow the manager. The header is re-resolved against the
// shell's note-owner population, since an archived project the archived facet
// loaded is never an owner and would only bounce back to All Notes.
func (m *AppModel) coreNotesScope() NotesScope {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return AllNotesScope()
	}
	header := out.cursorProjectHeaderProject()
	if header == nil || m.activeProjectByID(header.ID) == nil {
		return AllNotesScope()
	}
	return ProjectNotesScope(header.ID)
}

// coreCaptureMessage routes base capture to the workspace-owned System Inbox
// and project-focus capture to that focused project. A task Focus wins first
// and captures a direct child of the focus root, which is an explicit
// projection anchor pushed by Enter rather than the cursor. At the nesting cap
// it degrades to plain capture into the focus root's project. Cursor project
// identity is deliberately ignored at base scope, with one exception: a project
// header row names a project explicitly rather than incidentally, so capture
// there lands in it. cursorProjectHeaderProject returns nil on every task row,
// so the invariant holds verbatim everywhere else.
func (m *AppModel) coreCaptureMessage() tea.Msg {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return m.captureComposerMessage(nil, nil)
	}
	var project *models.Project
	if identity, focused := out.focusRootIdentity(); focused {
		if task, focusProject := m.taskAndProjectForIdentity(identity); task != nil && focusProject != nil {
			if taskAcceptsDeeperChild(task) {
				return ShowTaskComposerMsg{Preset: m.focusChildComposerPreset(focusProject, task)}
			}
			project = focusProject
		}
	}
	if project == nil {
		switch {
		case out.project != nil:
			project = out.project
		case out.isProjectScoped():
			project = out.scopedProject()
		}
	}
	if project == nil {
		project = out.cursorProjectHeaderProject()
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

// screenJumpPosition names the three screen-relative landings H/M/L ask for.
type screenJumpPosition int

const (
	screenJumpTop screenJumpPosition = iota
	screenJumpMiddle
	screenJumpBottom
)

// coreMoveCursorScreen lands the cursor on a position in the window the last
// render drew. It deliberately bypasses handleListNavigation, which is shared
// with Notes and knows nothing about a scroll window, and it never touches
// viewScrollOffset: the landing stays inside the window, so the next render
// leaves the scroll where it was.
func (m *AppModel) coreMoveCursorScreen(position screenJumpPosition) OperationHandlerResult {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return OperationHandlerResult{}
	}
	start, end := out.visibleRowWindow()
	if start >= end {
		return OperationHandlerResult{Status: "No row to jump to on screen"}
	}
	target, preferred := start, 1
	switch position {
	case screenJumpMiddle:
		target = start + windowMiddleIndex(end-start)
	case screenJumpBottom:
		target, preferred = end-1, -1
	case screenJumpTop:
	}
	landing, found := out.nearestNavigableInWindow(target, start, end, preferred)
	if !found {
		return OperationHandlerResult{Status: "No row to jump to on screen"}
	}
	previous := out.cursor
	out.cursor = landing
	cmd := flashIfJumped(previous, out.cursor, &out.cursorFlash)
	out.updateRangeSelection()
	return OperationHandlerResult{Message: commandMessage(cmd)}
}

// windowMiddleIndex is the offset of the middle row inside a window of the given
// height. M and zz read the same formula so the two families agree on "middle".
func windowMiddleIndex(height int) int { return (height - 1) / 2 }

// viewScrollAnchor names where in the window zt/zz/zb park the cursor's row. It
// is the inverse of screenJumpPosition, which names where in the window H/M/L
// park the cursor itself.
type viewScrollAnchor int

const (
	viewScrollAnchorTop viewScrollAnchor = iota
	viewScrollAnchorMiddle
	viewScrollAnchorBottom
)

// coreScrollCursorTo moves the window so the cursor's row sits at the requested
// position, leaving the cursor itself alone. The written offset still contains
// the cursor, which is exactly what adjustPersistentScrollOffset leaves
// untouched on the next render. Success is silent: a status here would arm the
// footer flash for a change already fully visible on screen.
func (m *AppModel) coreScrollCursorTo(anchor viewScrollAnchor) OperationHandlerResult {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return OperationHandlerResult{}
	}
	start, end := out.visibleRowWindow()
	if start >= end {
		return OperationHandlerResult{}
	}
	height := end - start
	desired := out.cursor
	switch anchor {
	case viewScrollAnchorMiddle:
		desired -= windowMiddleIndex(height)
	case viewScrollAnchorBottom:
		desired -= height - 1
	case viewScrollAnchorTop:
	}
	out.viewScrollOffset = min(max(desired, 0), max(len(out.rows)-height, 0))
	return OperationHandlerResult{}
}

// coreGotoTaskProject is the task-row counterpart to Enter on a project header:
// it scopes the Outliner to the cursor task's project and forces All so the
// whole tree is listed rather than the origin mode's narrowing. The scope push
// captures the frame Esc restores, so the mode switch must follow it — reversed,
// Esc would return to All instead of the mode the jump was pressed from. A mode
// that lists the task where All does not (a Done task reached from Recent) keeps
// the origin instead, and says so.
func (m *AppModel) coreGotoTaskProject(targets ResolvedOperationTargets) OperationHandlerResult {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return OperationHandlerResult{}
	}
	// Search is dismissed before the origin mode is read and before the scope
	// push, since that push captures the frame Esc restores and it must be the
	// pre-search one.
	out.ClearSearch()
	task, project := m.taskAndProjectForTargets(targets)
	if task == nil || project == nil {
		return OperationHandlerResult{}
	}
	origin := out.activeOutlinerModeEntry()
	previous := out.cursor

	out.pushProjectScope(project.ID)
	out.selectOutlinerMode(ModeAll)
	out.NavigateToTaskInProject(task, project)

	status := "Project: " + project.Name
	if _, found := out.rowIndexForTask(taskIdentityFor(task, project)); !found && origin.ID != ModeAll {
		out.selectOutlinerMode(origin.ID)
		out.NavigateToTaskInProject(task, project)
		status += " — kept " + origin.Label + ", the task is not listed in All"
	}
	m.setInfoStatus(status)
	return OperationHandlerResult{
		Message: commandMessage(tea.Batch(
			flashIfJumped(previous, out.cursor, &out.cursorFlash),
			defaultStatusTimeout(),
		)),
	}
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

// coreReorderEdge is the single-press equivalent of repeating coreReorder until
// the row can move no further, and takes the same ordering-context dispatch so
// the two families can never disagree about which durable order a row belongs to.
func (m *AppModel) coreReorderEdge(delta int) tea.Cmd {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil || !out.simplifiedReorderEligible() {
		if out != nil {
			out.orderingUnavailableStatus()
		}
		return nil
	}
	if out.isPinnedReorderRow() {
		return out.pinnedReorderEdge(delta)
	}
	if out.isPlanningReorderMode() {
		_, cmd := out.planningReorderEdge(delta)
		return cmd
	}
	return out.reorderTaskEdge(delta)
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

// coreInputHelpItems documents the Outliner from the two grammars a user meets
// there: the registry's keys and the shared sigil language they type into
// capture, inline edit, and search. The grammar rows are appended here rather
// than inside HelpItemsForInputStrategy so that stays a pure registry
// projection and the grammar section stays Outliner-scoped.
func (m *AppModel) coreInputHelpItems() []HelpItem {
	if m.operationRegistry == nil {
		return nil
	}
	items := m.operationRegistry.HelpItemsForInputStrategy(
		InputStrategySimplified,
		m.operationCommandContext(),
	)
	return append(items, grammarHelpItems()...)
}
