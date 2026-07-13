package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// handleConflictMessages routes the conflict-management surfaces: opening the
// resolution view from a notification entry point, showing a specific conflict,
// and reacting to a completed resolution.
func (m *AppModel) handleConflictMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case OpenConflictsMsg:
		return m.openNewestConflict()

	case ShowConflictResolutionMsg:
		return m.showViewInit(ViewConflictResolution, NewConflictResolutionModel(m.config, m.configManager, msg.Bundle))

	case ConflictResolvedMsg:
		m.popView()
		switch {
		case msg.NewConflictID != "":
			m.setInfoStatus("Remote changed during resolution; new conflict recorded (" + shortConflictNoticeID(msg.NewConflictID) + ")")
		case msg.Adopted:
			m.setSuccessStatus("Conflict resolved: adopted remote state")
		default:
			m.setSuccessStatus("Conflict resolved")
		}
		return true, nil
	}
	return false, nil
}

// OpenConflictsMsg is the notification entry point: it opens the newest
// unresolved conflict, or reports that there are none.
type OpenConflictsMsg struct{}

// openNewestConflict opens the resolution view for the most recently recorded
// conflict, or sets an informational status when the workspace is conflict-free.
func (m *AppModel) openNewestConflict() (bool, tea.Cmd) {
	store := m.conflictStore()
	records, err := store.List()
	if err != nil {
		m.setInfoStatus("Could not read conflicts: " + err.Error())
		return true, nil
	}
	if len(records) == 0 {
		m.setInfoStatus("No unresolved conflicts")
		return true, nil
	}
	bundle, err := store.Load(records[0].ID)
	if err != nil {
		m.setInfoStatus("Could not load conflict: " + err.Error())
		return true, nil
	}
	return m.showViewInit(ViewConflictResolution, NewConflictResolutionModel(m.config, m.configManager, bundle))
}

// conflictStore builds the conflict store for the active workspace.
func (m *AppModel) conflictStore() *storage.ConflictStore {
	dataDir := config.WorkspaceRootFromConfigPath(m.configManager.GetConfigPath())
	return storage.NewConflictStore(config.ConflictsDir(dataDir))
}

// shortConflictNoticeID trims a conflict id for a compact status message.
func shortConflictNoticeID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
