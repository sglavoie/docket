package tui

import (
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

	case DiscoveryListSelectMsg:
		return m.handleDiscoveryListSelection(msg)

	}
	return false, nil
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
		popCmd := m.popView()
		if target.Tool == nil {
			return true, popCmd
		}
		navigationCmd := m.openFinderTool(target.Tool.ID)
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
		outliner.activeSavedFilter = ""
		outliner.rebuildRows()
		return true, m.returnToOutlinerAfterContextSelection()

	case FinderItemSavedFilter:
		if target.SavedFilter == nil {
			return true, m.popView()
		}
		outliner := m.discoveryOutliner()
		if outliner == nil {
			return true, m.popView()
		}
		m.applySavedFilterToOutlinerModel(outliner, *target.SavedFilter)
		return true, m.returnToOutlinerAfterContextSelection()
	}
	return true, m.popView()
}

func (m *AppModel) openFinderTool(id string) tea.Cmd {
	switch id {
	case "weekly-review":
		_, cmd := m.handleViewMessages(ShowWeeklyReviewMsg{})
		return cmd
	case "metrics":
		_, cmd := m.handleViewMessages(ShowMetricsDashboardMsg{})
		return cmd
	case "roadmap":
		_, cmd := m.handleViewMessages(ShowRoadmapMsg{})
		return cmd
	case "blockers":
		_, cmd := m.handleViewMessages(ShowGlobalBlockersMsg{})
		return cmd
	case "notes":
		return m.activateManageHub(ManageParams{Module: ManageNotes, Scope: AppEntityScope()})
	case "tags":
		_, cmd := m.handleTagMessages(ShowGlobalTagManagerMsg{})
		return cmd
	case "workflow-rules":
		return m.activateManageHub(ManageParams{Module: ManageRules, Scope: AppEntityScope()})
	case "settings":
		return m.activateManageHub(ManageParams{Module: ManageSettings, Scope: AppEntityScope()})
	case "contexts":
		return m.activateManageHub(ManageParams{Module: ManageContexts, Scope: AppEntityScope()})
	default:
		return func() tea.Msg { return ErrorMsg{Err: ErrListItemNotFound} }
	}
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

func (m *AppModel) discoveryOutliner() *OutlinerModel {
	if outliner, _, ok := m.outlinerPrimary(); ok {
		return outliner
	}
	return m.crossOutliner
}

func (m *AppModel) selectFinderNote(filename string) {
	if m.manageHub == nil {
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
