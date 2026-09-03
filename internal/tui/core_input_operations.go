package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/status"
)

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
		operationCoreClearFolds,
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
					(m.crossOutliner == nil || !m.crossOutliner.showsRankedList())
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
			message = chrome.ShowTaskTextEditorMsg{
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
			message = commandMessage(status.DefaultStatusTimeout())
		case project == nil:
			m.setWarningStatus("No project under the cursor to outline")
			message = commandMessage(status.DefaultStatusTimeout())
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
			message = commandMessage(status.DefaultStatusTimeout())
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
	// The one operation reached only from the menu bar reports both ways, unlike
	// the three toggles beside it: a user who ran it off a menu row watched the
	// dropdown close over the rows it changed, so a silent no-op would read as the
	// menu having eaten the press.
	case operationCoreClearFolds:
		if out, ok := m.outlinerPrimary(); ok {
			if !out.clearAllFolds() {
				return OperationHandlerResult{Status: "Nothing is folded"}
			}
			return OperationHandlerResult{Status: "All folds cleared"}
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
	case operationCoreOpenNote:
		// All Notes rather than the Outliner's own scope: the manager has to hold
		// the note the row named whatever the search was scoped to, and Focus is
		// what positions the cursor on it once it is there.
		if out, ok := m.outlinerPrimary(); ok {
			if identity, onNote := out.cursorNoteIdentity(); onNote {
				message = ShowNotesManagerMsg{Scope: AllNotesScope(), Focus: identity}
			}
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
	case operationCoreSelectPinned:
		message = ShowProjectHubMsg{Mode: ProjectHubModePinned}
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
	case operationCoreNamedLenses:
		// The picker is a shell overlay rather than a route, so opening it mounts
		// the model here and returns whatever command that mount raised.
		message = commandMessage(m.openNamedLensPicker())
	case operationCoreConflicts:
		// Like the picker the browser is a shell overlay rather than a route, and
		// the mount itself decides whether there is anything to show.
		message = m.openConflictBrowser()
	case operationCoreQueryReadout:
		if out, ok := m.outlinerPrimary(); ok {
			out.toggleQueryReadout()
		}
	case operationCoreNotes:
		message = ShowNotesManagerMsg{Scope: m.coreNotesScope()}
	case operationCoreToggleTheme:
		err = m.coreToggleTheme()
		if err == nil {
			message = commandMessage(status.DefaultStatusTimeout())
		}
	case operationCoreToggleMenuBar:
		err = m.coreToggleMenuBar()
		if err == nil {
			message = commandMessage(status.DefaultStatusTimeout())
		}
	case operationCoreToggleAdvisories:
		err = m.coreToggleAdvisories()
		if err == nil {
			message = commandMessage(status.DefaultStatusTimeout())
		}
	case operationCoreOpenMenuBar:
		err = m.coreOpenMenuBar()
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
		message = status.QuitMsg{}
	case operationCoreNextWorkMode:
		if out, ok := m.outlinerPrimary(); ok {
			_, _ = out.cycleWorkMode(1)
		}
	case operationCorePreviousWorkMode:
		if out, ok := m.outlinerPrimary(); ok {
			_, _ = out.cycleWorkMode(-1)
		}
	case operationCoreResetToLaunchView:
		return m.coreResetToLaunchView()
	default:
		if mode, ok := gotoModeOperations[invocation.Descriptor.ID]; ok {
			if out, outlinerOK := m.outlinerPrimary(); outlinerOK {
				_, _ = out.selectOutlinerMode(mode)
			}
		}
	}
	return OperationHandlerResult{Message: message, Err: err}
}
