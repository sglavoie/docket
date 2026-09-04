package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

// coreInputOperationRefusesSearchProjection names the outliner navigation
// operations whose projection the temporary search replacement owns, so they
// refuse instead of reshaping a projection the user has not exited. The two
// Focus jumps and Go to Task's Project are deliberately absent: leaving the
// projection is what they are for, so they exit search onto the result instead.
func coreInputOperationRefusesSearchProjection(id operation.OperationID) bool {
	if _, ok := operation.GotoModeOperations[id]; ok {
		return true
	}
	switch id { //nolint:exhaustive // Only outliner navigation operations override applicability.
	case operation.OperationCoreNextWorkMode,
		operation.OperationCorePreviousWorkMode,
		operation.OperationCoreFocusLatest,
		operation.OperationCoreCollapseParent,
		operation.OperationCoreExpand,
		operation.OperationCoreToggleSections,
		operation.OperationCoreToggleProjectGroup,
		operation.OperationCoreToggleSubtrees,
		operation.OperationCoreClearFolds,
		operation.OperationCoreReorderDown,
		operation.OperationCoreReorderUp,
		operation.OperationCoreReorderBottom,
		operation.OperationCoreReorderTop:
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
	for _, descriptor := range operation.CoreInputOperationDescriptors() {
		if coreInputOperationRefusesSearchProjection(descriptor.ID) {
			inner := descriptor.Applicability
			descriptor.Applicability = func(ctx operation.CommandContext) bool {
				return (inner == nil || inner(ctx)) &&
					(m.crossOutliner == nil || !m.crossOutliner.ShowsRankedList())
			}
		}
		if err := m.registerOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities:     operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
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
func (m *AppModel) executeCoreInputOperation(invocation operation.OperationInvocation) operation.OperationHandlerResult {
	if operation.CoreInputOperationRequiresTask(invocation.Descriptor.ID) &&
		!invocation.Context.Selection.Visual &&
		!operation.CurrentTaskIdentity(invocation.Context).Valid() {
		return operation.OperationHandlerResult{Err: operation.OperationTargetRejection(invocation.Descriptor)}
	}

	var message tea.Msg
	var err error
	//nolint:exhaustive // Only core input operations are registered with this handler.
	switch invocation.Descriptor.ID {
	case operation.OperationCoreCapture:
		message = m.coreCaptureMessage()
	case operation.OperationCoreEdit:
		if task, project := m.taskAndProjectForTargets(invocation.Targets); task != nil {
			message = outliner.StartTaskLineEditMsg{Task: task, Project: project}
		}
	case operation.OperationCoreEditDescription:
		if task, project := m.taskAndProjectForTargets(invocation.Targets); task != nil {
			message = chrome.ShowTaskTextEditorMsg{
				Task: task, Project: project, Operation: app.TaskOperationEditDescription,
			}
		}
	case operation.OperationCoreExternalEditor:
		if task, project := m.taskAndProjectForTargets(invocation.Targets); task != nil {
			message = EditTaskContentMsg{Task: task, Project: project}
		}
	case operation.OperationCoreOutlineEditor:
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
	case operation.OperationCoreBacklog:
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		if invocation.Context.Selection.Visual {
			message = commandMessage(m.corePlanningOperation(invocation.Targets, app.PlanningBacklog, capture))
		} else {
			message = commandMessage(m.coreMoveToBacklogSingle(capture))
		}
	case operation.OperationCoreToday:
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		if invocation.Context.Selection.Visual {
			message = commandMessage(m.coreToggleToday(invocation.Targets, capture))
		} else {
			message = commandMessage(m.toggleToday(capture))
		}
	case operation.OperationCoreNext:
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		if invocation.Context.Selection.Visual {
			message = commandMessage(m.coreToggleNext(invocation.Targets, capture))
		} else {
			message = commandMessage(m.toggleQueue(capture))
		}
	case operation.OperationCoreActions:
		if actionErr := m.openTaskActionSheet(invocation.Context); actionErr != nil {
			m.setInfoStatus(actionErr.Error())
			message = commandMessage(status.DefaultStatusTimeout())
		}
	case operation.OperationCoreCursorDown:
		message = commandMessage(m.coreMoveCursor("j"))
	case operation.OperationCoreCursorUp:
		message = commandMessage(m.coreMoveCursor("k"))
	case operation.OperationCoreCursorTop:
		message = commandMessage(m.coreMoveCursor("home"))
	case operation.OperationCoreCursorBottom:
		message = commandMessage(m.coreMoveCursor("G"))
	case operation.OperationCoreCursorPageUp:
		message = commandMessage(m.coreMoveCursor("ctrl+u"))
	case operation.OperationCoreCursorPageDown:
		message = commandMessage(m.coreMoveCursor("ctrl+d"))
	case operation.OperationCoreScrollCursorTop:
		return m.coreScrollCursorTo(viewScrollAnchorTop)
	case operation.OperationCoreScrollCursorMiddle:
		return m.coreScrollCursorTo(viewScrollAnchorMiddle)
	case operation.OperationCoreScrollCursorBottom:
		return m.coreScrollCursorTo(viewScrollAnchorBottom)
	case operation.OperationCoreNextHeader, operation.OperationCorePreviousHeader:
		delta := 1
		if invocation.Descriptor.ID == operation.OperationCorePreviousHeader {
			delta = -1
		}
		if out, ok := m.outlinerPrimary(); ok {
			cmd, moved := out.MoveCursorToAdjacentHeader(delta)
			if !moved {
				return operation.OperationHandlerResult{Status: "No sections to jump between"}
			}
			message = commandMessage(cmd)
		}
	case operation.OperationCoreJumpBackward:
		if out, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.JumpBackward())
		}
	case operation.OperationCoreJumpForward:
		if out, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.JumpForward())
		}
	case operation.OperationCoreFocusLatest:
		if out, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.FocusLatestActivity())
		}
	case operation.OperationCoreGotoTaskProject:
		return m.coreGotoTaskProject(invocation.Targets)
	case operation.OperationCoreCollapseParent:
		if out, ok := m.outlinerPrimary(); ok {
			out.CollapseOrMoveToParent()
		}
	case operation.OperationCoreExpand:
		if out, ok := m.outlinerPrimary(); ok {
			out.ExpandAtCursor()
		}
	// The three fold toggles report their refusals and nothing else: a productive
	// press returns a zero result, so it arms no footer flash for a change already
	// on screen. p names P because it always refuses inside a project scope, which
	// is exactly where the key used to open Select Project.
	case operation.OperationCoreToggleSections:
		if out, ok := m.outlinerPrimary(); ok {
			if !out.ToggleVisibleSections() {
				return operation.OperationHandlerResult{Status: "No foldable sections here"}
			}
		}
	case operation.OperationCoreToggleProjectGroup:
		if out, ok := m.outlinerPrimary(); ok {
			if !out.ToggleVisibleProjectGroups() {
				return operation.OperationHandlerResult{Status: "No project groups here; P switches project"}
			}
		}
	case operation.OperationCoreToggleSubtrees:
		if out, ok := m.outlinerPrimary(); ok {
			if !out.ToggleAllSubtrees() {
				return operation.OperationHandlerResult{Status: "No foldable subtrees here"}
			}
		}
	// The one operation reached only from the menu bar reports both ways, unlike
	// the three toggles beside it: a user who ran it off a menu row watched the
	// dropdown close over the rows it changed, so a silent no-op would read as the
	// menu having eaten the press.
	case operation.OperationCoreClearFolds:
		if out, ok := m.outlinerPrimary(); ok {
			if !out.ClearAllFolds() {
				return operation.OperationHandlerResult{Status: "Nothing is folded"}
			}
			return operation.OperationHandlerResult{Status: "All folds cleared"}
		}
	case operation.OperationCoreReorderDown:
		message = commandMessage(m.coreReorder(1))
	case operation.OperationCoreReorderUp:
		message = commandMessage(m.coreReorder(-1))
	case operation.OperationCoreReorderBottom:
		message = commandMessage(m.coreReorderEdge(1))
	case operation.OperationCoreReorderTop:
		message = commandMessage(m.coreReorderEdge(-1))
	case operation.OperationCoreFocusSubtree:
		if out, ok := m.outlinerPrimary(); ok {
			if status, landed := out.ExitSearchOntoCursorTask(); !landed {
				return operation.OperationHandlerResult{Status: status}
			}
			if !out.FocusSubtreeAtCursor() {
				return operation.OperationHandlerResult{Status: "No children to focus"}
			}
		}
	case operation.OperationCoreFocusParent:
		if out, ok := m.outlinerPrimary(); ok {
			if status, landed := out.ExitSearchOntoCursorTask(); !landed {
				return operation.OperationHandlerResult{Status: status}
			}
			if !out.FocusParentAtCursor() {
				return operation.OperationHandlerResult{Status: "No parent to focus"}
			}
		}
	case operation.OperationCoreZoomSection:
		if out, ok := m.outlinerPrimary(); ok {
			out.ZoomSectionAtCursor()
		}
	case operation.OperationCoreActivateGroup:
		if out, ok := m.outlinerPrimary(); ok {
			out.ActivateGroupAtCursor()
		}
	case operation.OperationCoreOpenNote:
		// All Notes rather than the Outliner's own scope: the manager has to hold
		// the note the row named whatever the search was scoped to, and Focus is
		// what positions the cursor on it once it is there.
		if out, ok := m.outlinerPrimary(); ok {
			if identity, onNote := out.CursorNoteIdentity(); onNote {
				message = notes.ShowNotesManagerMsg{Scope: route.AllNotesScope(), Focus: identity}
			}
		}
	case operation.OperationCorePeek:
		if out, ok := m.outlinerPrimary(); ok {
			out.TogglePeek()
		}
	case operation.OperationCoreScopeSearch:
		if out, ok := m.outlinerPrimary(); ok {
			message = commandMessage(out.ActivateSearch())
		}
	case operation.OperationCoreSelectProject:
		message = projecthub.ShowProjectHubMsg{}
	case operation.OperationCoreSelectPinned:
		message = projecthub.ShowProjectHubMsg{Mode: route.ProjectHubModePinned}
	case operation.OperationCoreMultiSelect:
		if out, ok := m.outlinerPrimary(); ok {
			out.ToggleCursorSelection()
		}
	case operation.OperationCoreRangeSelect:
		if out, ok := m.outlinerPrimary(); ok {
			out.StartCursorRangeSelection()
		}
	case operation.OperationCoreSelectAll:
		if out, ok := m.outlinerPrimary(); ok {
			out.SelectAllVisible()
			out.UpdateVisualSelectionStatus()
		}
	case operation.OperationCoreNamedLenses:
		// The picker is a shell overlay rather than a route, so opening it mounts
		// the model here and returns whatever command that mount raised.
		message = commandMessage(m.openNamedLensPicker())
	case operation.OperationCoreConflicts:
		// Like the picker the browser is a shell overlay rather than a route, and
		// the mount itself decides whether there is anything to show.
		message = m.openConflictBrowser()
	case operation.OperationCoreQueryReadout:
		if out, ok := m.outlinerPrimary(); ok {
			out.ToggleQueryReadout()
		}
	case operation.OperationCoreNotes:
		message = notes.ShowNotesManagerMsg{Scope: m.coreNotesScope()}
	case operation.OperationCoreToggleTheme:
		err = m.coreToggleTheme()
		if err == nil {
			message = commandMessage(status.DefaultStatusTimeout())
		}
	case operation.OperationCoreToggleMenuBar:
		err = m.coreToggleMenuBar()
		if err == nil {
			message = commandMessage(status.DefaultStatusTimeout())
		}
	case operation.OperationCoreToggleAdvisories:
		err = m.coreToggleAdvisories()
		if err == nil {
			message = commandMessage(status.DefaultStatusTimeout())
		}
	case operation.OperationCoreOpenMenuBar:
		err = m.coreOpenMenuBar()
	case operation.OperationCoreUndo:
		message = commandMessage(m.coreUndo())
	case operation.OperationCoreRedo:
		message = commandMessage(m.coreRedo())
	case operation.OperationCoreBack:
		if out, ok := m.outlinerPrimary(); ok {
			listsurface.RunEscCascade(out.EscSteps())
			// Leaving a parked focus drops the project from the snapshot in place;
			// the cascade cannot return a Cmd, so the reload is claimed here.
			message = commandMessage(out.TakeParkedFocusReload())
		}
	case operation.OperationCoreHelp:
		message = help.ShowHelpMsg{
			ViewName: "Outliner",
			Items:    m.coreInputHelpItems(),
		}
	case operation.OperationCoreQuit:
		message = status.QuitMsg{}
	case operation.OperationCoreNextWorkMode:
		if out, ok := m.outlinerPrimary(); ok {
			_, _ = out.CycleWorkMode(1)
		}
	case operation.OperationCorePreviousWorkMode:
		if out, ok := m.outlinerPrimary(); ok {
			_, _ = out.CycleWorkMode(-1)
		}
	case operation.OperationCoreResetToLaunchView:
		return m.coreResetToLaunchView()
	default:
		if mode, ok := operation.GotoModeOperations[invocation.Descriptor.ID]; ok {
			if out, outlinerOK := m.outlinerPrimary(); outlinerOK {
				_, _ = out.SelectOutlinerMode(mode)
			}
		}
	}
	return operation.OperationHandlerResult{Message: message, Err: err}
}
