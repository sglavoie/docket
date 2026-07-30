package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

// handleTaskLinkMessages handles task link-related messages (linked tasks, cross-project links).
// Returns true if the message was handled.
func (m *AppModel) handleTaskLinkMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowCrossProjectLinkerMsg:
		return m.handleShowCrossProjectLinker(msg)

	case CrossProjectLinkCreatedMsg:
		return m.handleCrossProjectLinkCreated(msg)
	}
	return false, nil
}

// handleShowCrossProjectLinker handles the ShowCrossProjectLinkerMsg.
func (m *AppModel) handleShowCrossProjectLinker(msg ShowCrossProjectLinkerMsg) (bool, tea.Cmd) {
	if msg.SourceTask != nil && msg.SourceProject != nil {
		linker := NewCrossProjectLinkerModel(msg.SourceTask, msg.SourceProject, m.runtime, m.secondaryFeatures.Link)
		m.showView(ViewCrossProjectLinker, linker)
	}
	return true, nil
}

// handleCrossProjectLinkCreated handles the CrossProjectLinkCreatedMsg.
func (m *AppModel) handleCrossProjectLinkCreated(msg CrossProjectLinkCreatedMsg) (bool, tea.Cmd) {
	m.popView()
	m.refreshAllViews()
	// Determine link type name for status message
	var linkName string
	switch msg.LinkType {
	case LinkTypeBlocker:
		linkName = "blocker"
	case LinkTypeDependent:
		linkName = "dependency"
	default:
	}
	m.setSuccessStatus(fmt.Sprintf("Linked #%d → %s #%d (%s)", msg.SourceTask.ShortID, msg.TargetProject.Name, msg.TargetTask.ShortID, linkName))
	return true, statusTimeout(3 * time.Second)
}
