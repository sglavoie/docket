package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

// handleSearchMessages handles Palette search and selection messages.
// Returns true if the message was handled.
func (m *AppModel) handleSearchMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowFinderMsg:
		context := m.operationCommandContext()
		finder, err := NewRuntimeFinder(m.runtime, m.GetOperationRegistry(), context)
		if err != nil {
			return true, func() tea.Msg { return ErrorMsg{Err: err} }
		}
		discovery, err := NewFinderDiscoveryModel(finder, context, msg.PreFilter)
		if err != nil {
			return true, func() tea.Msg { return ErrorMsg{Err: err} }
		}
		return m.showViewInit(ViewFinder, discovery)

	case ShowModeListMsg:
		discovery, err := NewModeDiscoveryModel(m.operationCommandContext())
		if err != nil {
			return true, func() tea.Msg { return ErrorMsg{Err: err} }
		}
		return m.showViewInit(ViewFinder, discovery)

	case CloseFinderMsg:
		return true, m.popView()

	case ActivateOutlinerModeMsg:
		return m.activateDiscoveryMode(msg.Mode)

	case ShowFinderContextActionsMsg:
		return true, m.openFinderContextActionSheet(msg)

	case ContextActionSheetCompletedMsg:
		if msg.Host != ContextActionHostFinder {
			return false, nil
		}
		return true, batchPaletteExecuteCmds(m.closeFinderRoute(), msg.Cmd)

	case DiscoveryListSelectMsg:
		return m.handleDiscoveryListSelection(msg)

	}
	return false, nil
}

// openFinderContextActionSheet freezes the selected row as the sole target, so
// a later result rebuild or cursor move cannot retarget the pending action.
func (m *AppModel) openFinderContextActionSheet(msg ShowFinderContextActionsMsg) tea.Cmd {
	target, ok := msg.Item.Data.(FinderTarget)
	if !ok {
		return m.reportFinderActionsUnavailable()
	}
	sheet, err := m.buildFinderContextActionSheet(msg, target)
	if err != nil {
		m.err = err
		return statusTimeout(5 * time.Second)
	}
	// An empty sheet means every advertised action was dropped as inapplicable,
	// which is a status message rather than an unusable modal.
	if sheet == nil || len(sheet.operationIDs()) == 0 {
		return m.reportFinderActionsUnavailable()
	}
	m.contextActionSheet = sheet
	return nil
}

func (m *AppModel) buildFinderContextActionSheet(
	msg ShowFinderContextActionsMsg,
	target FinderTarget,
) (*ContextActionSheetModel, error) {
	frozen, ok := finderContextActionTarget(msg.Context, target)
	if !ok {
		return nil, nil
	}
	if target.Kind == FinderItemTask {
		return NewFinderTaskActionSheetModel(
			m.GetOperationRegistry(),
			m.operationExecutor,
			frozen.context,
			frozen.subtitle,
			msg.Item.OperationIDs,
		)
	}
	return NewFinderContextActionSheetModel(
		m.GetOperationRegistry(),
		m.operationExecutor,
		frozen,
		msg.Item.OperationIDs,
	)
}

func (m *AppModel) reportFinderActionsUnavailable() tea.Cmd {
	m.setInfoStatus("No actions are available for this result")
	return defaultStatusTimeout()
}

// closeFinderRoute unwinds the Finder once after a completed contextual action.
// The route check keeps it idempotent: a follow-up completion cannot pop the
// destination the first one opened.
func (m *AppModel) closeFinderRoute() tea.Cmd {
	if m.currentView() != ViewFinder {
		return nil
	}
	return m.popView()
}

func (m *AppModel) handleDiscoveryListSelection(msg DiscoveryListSelectMsg) (bool, tea.Cmd) {
	switch target := msg.Item.Data.(type) {
	case FinderTarget:
		return m.handleFinderTargetSelection(msg.Item.ID, target, msg.Context)
	case OutlinerMode:
		m.recordFinderDestination(config.RecentDestination{
			Kind: config.RecentDestinationMode,
			ID:   string(target.ID),
		})
		return m.activateDiscoveryMode(target.ID)
	default:
		return true, func() tea.Msg {
			return ErrorMsg{Err: ErrListItemNotFound}
		}
	}
}

func (m *AppModel) handleFinderTargetSelection(
	itemID ListItemID,
	target FinderTarget,
	context CommandContext,
) (bool, tea.Cmd) {
	if destination, ok := target.RecentDestination(); ok &&
		destination.QualifiedID() == string(itemID) {
		m.recordFinderDestination(destination)
	}

	switch target.Kind {
	case FinderItemOperation:
		if target.Operation == nil || m.operationExecutor == nil {
			return true, func() tea.Msg { return ErrorMsg{Err: ErrOperationNotRegistered} }
		}
		popCmd := m.popView()
		actionCmd := m.executeOperationOverlayRequest(OperationExecutionRequest{
			ID:      target.Operation.ID,
			Context: cloneCommandContext(context),
			Source:  OperationSourceFinder,
		})
		return true, batchPaletteExecuteCmds(popCmd, actionCmd)

	case FinderItemTask:
		popCmd := m.popView()
		if target.Task == nil || target.Project == nil {
			return true, popCmd
		}
		m.recordTaskVisit(target.Task, target.Project)
		return true, batchPaletteExecuteCmds(
			popCmd,
			m.navigateToTaskInProject(target.Task, target.Project),
		)

	case FinderItemProject:
		popCmd := m.popView()
		_, selectCmd := m.handleProjectMessages(SelectProjectMsg{Project: target.Project})
		return true, batchPaletteExecuteCmds(popCmd, selectCmd)

	case FinderItemNote:
		popCmd := m.popView()
		if target.Note == nil {
			return true, popCmd
		}
		params := ManageParams{Module: ManageNotes, Scope: AppEntityScope()}
		if target.Project != nil {
			params.Scope = ProjectEntityScope(target.Project.ID)
		}
		openCmd := m.activateManageHub(params)
		m.selectFinderNote(target.Note.Filename)
		return true, batchPaletteExecuteCmds(popCmd, openCmd)

	case FinderItemTool:
		if target.Tool == nil || m.operationExecutor == nil {
			return true, m.popView()
		}
		popCmd := m.popView()
		navigationCmd := m.executeOperationOverlayRequest(OperationExecutionRequest{
			ID:      OperationID(target.Tool.ID),
			Context: cloneCommandContext(context),
			Source:  OperationSourceFinder,
		})
		return true, batchPaletteExecuteCmds(popCmd, navigationCmd)

	case FinderItemMode:
		if target.Mode == nil {
			return true, m.popView()
		}
		return m.activateDiscoveryMode(target.Mode.ID)

	case FinderItemScope:
		if target.Scope == nil {
			return true, m.popView()
		}
		scope := target.Scope.Scope
		return m.handleContextMessages(ContextScopeSelectedMsg{
			SelectedContext: scope.Name,
			IsCustomContext: scope.IsCustom(),
		})

	case FinderItemTag:
		if target.Tag == nil {
			return true, m.popView()
		}
		outliner := m.discoveryOutliner()
		if outliner == nil {
			return true, m.popView()
		}
		if outliner.filter.Tags == nil {
			outliner.filter.Tags = make(map[string]bool)
		}
		outliner.filter.Tags[target.Tag.Name] = true
		outliner.rebuildRows()
		return true, m.returnToOutlinerAfterContextSelection()
	}
	return true, m.popView()
}

func (m *AppModel) activateDiscoveryMode(id OutlinerModeID) (bool, tea.Cmd) {
	outliner := m.discoveryOutliner()
	if outliner == nil {
		return true, m.popView()
	}
	_, modeCmd := outliner.selectOutlinerMode(id)
	return true, batchPaletteExecuteCmds(
		m.returnToOutlinerAfterContextSelection(),
		modeCmd,
	)
}

// refreshFinderItems rebuilds the open Finder's providers so a mutation made
// from one of its rows is reflected without reopening the route.
func (m *AppModel) refreshFinderItems() {
	if m.currentView() != ViewFinder {
		return
	}
	if discovery, ok := m.views.Get(ViewFinder).(*DiscoveryListModel); ok &&
		discovery.kind == DiscoverySurfaceFinder {
		finder, err := NewRuntimeFinder(m.runtime, m.GetOperationRegistry(), discovery.context)
		if err != nil {
			return
		}
		discovery.finder = finder
		_ = discovery.rebuildFinderResults()
	}
}

func (m *AppModel) discoveryOutliner() *OutlinerModel {
	if outliner, _, ok := m.outlinerPrimary(); ok {
		return outliner
	}
	return m.crossOutliner
}

func (m *AppModel) selectFinderNote(filename string) {
	if filename == "" || m.manageHub == nil {
		return
	}
	notes, ok := m.manageHub.ActiveModuleView().(*NotesViewModel)
	if !ok {
		return
	}
	for index, note := range notes.notes {
		if note != nil && note.Filename == filename {
			notes.cursor = index
			return
		}
	}
}

func (m *AppModel) recordFinderDestination(destination config.RecentDestination) {
	if !destination.Valid() {
		return
	}
	if err := m.commitMachineMutation(func(machine *config.MachineState) {
		machine.RecordRecentDestination(destination)
	}); err != nil {
		logging.Warn("failed to save Finder destination: %v", err)
	}
}

// batchPaletteExecuteCmds combines the popView cmd and the palette result cmd,
// dropping nils so a single non-nil cmd is returned without wrapping it in a
// degenerate Batch.
func batchPaletteExecuteCmds(popCmd, resultCmd tea.Cmd) tea.Cmd {
	switch {
	case popCmd == nil && resultCmd == nil:
		return nil
	case popCmd == nil:
		return resultCmd
	case resultCmd == nil:
		return popCmd
	default:
		return tea.Batch(popCmd, resultCmd)
	}
}
