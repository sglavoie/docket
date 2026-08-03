package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// handleContextMessages handles Context Hub navigation and the outcomes of the
// hub's context mutations. Returns true if the message was handled.
func (m *AppModel) handleContextMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowContextHubMsg:
		return true, m.activateManageHub(ManageParams{Module: ManageContexts})

	case ShowProjectHubMsg:
		return true, m.activateManageHub(ManageParams{Module: ManageProjects})

	case scopeHubInlineActionMsg:
		return true, m.openScopeHubForInlineAction(msg)

	case ContextScopeSelectedMsg:
		if msg.Canceled {
			return true, m.popView()
		}
		scope := RegularContextScope(msg.SelectedContext)
		if m.crossOutliner != nil {
			m.crossOutliner.applyContextScope(scope)
		}
		// Applying a scope is navigation to the scoped Outliner, not merely a
		// dismissal of the Context Hub. In particular, the global palette command
		// can open the hub above another child; one pop would reveal that child
		// again and could leave the user outside the new scope.
		return true, tea.Batch(m.returnToOutlinerAfterContextSelection(), m.requestCrossProjectOutlinerReload())

	case RegularContextSavedMsg:
		// Regular labels live on the catalog projects, so the cached projects the
		// rest of the shell renders from carry a stale Context until this resync.
		m.syncProjectCache()
		m.setSuccessStatus(regularContextSavedStatus(msg))
		return true, tea.Batch(defaultStatusTimeout(),
			m.reconcileRegularContextScope(msg.WasActive, RegularContextScope(msg.Name)))

	case RegularContextDeletedMsg:
		m.syncProjectCache()
		m.setSuccessStatus(fmt.Sprintf("Deleted context '%s': %d project(s) moved to '%s'",
			msg.Name, msg.ProjectCount, msg.Destination))
		return true, tea.Batch(defaultStatusTimeout(),
			m.reconcileRegularContextScope(msg.WasActive, AllContextScope()))
	}
	return false, nil
}

// openScopeHubForInlineAction routes a custom-scope mutation raised outside the
// Scope Hub into a hub opened on the exact frozen scope. The hub intercepts its
// own inline actions before they reach the shell, so this only ever runs for a
// scope action started somewhere else, such as a Scope Hub row.
func (m *AppModel) openScopeHubForInlineAction(msg scopeHubInlineActionMsg) tea.Cmd {
	cmd := m.activateManageHub(ManageParams{Module: ManageContexts})
	if m.manageHub == nil {
		return cmd
	}
	hub, ok := m.manageHub.ActiveModuleView().(*ContextHubModel)
	if !ok {
		return cmd
	}
	hub.applyScopeHubInlineAction(msg)
	return cmd
}

// returnToOutlinerAfterContextSelection dismisses every primary child above the
// sole Outliner root without resetting its freshly applied context scope.
// Cancel uses the ordinary one-pop path instead so it can restore the prior view.
func (m *AppModel) returnToOutlinerAfterContextSelection() tea.Cmd {
	m.navigation.ResetRoot()
	if view := m.views.Get(m.currentView()); view != nil {
		m.setAccentOnView(view)
	}
	return func() tea.Msg { return ViewDidAppearMsg{} }
}

func regularContextSavedStatus(msg RegularContextSavedMsg) string {
	if msg.IsNew {
		return fmt.Sprintf("Created context '%s' with %d project(s)", msg.Name, msg.Assigned)
	}
	if msg.Renamed {
		return fmt.Sprintf("Renamed '%s' to '%s' (%d project(s) relabeled)",
			msg.PreviousName, msg.Name, msg.RelabeledCount)
	}
	if msg.Assigned == 0 && msg.Removed == 0 {
		return fmt.Sprintf("Context '%s' is unchanged", msg.Name)
	}
	if msg.Removed > 0 {
		return fmt.Sprintf("Context '%s': %d added, %d moved to '%s'",
			msg.Name, msg.Assigned, msg.Removed, msg.Destination)
	}
	return fmt.Sprintf("Context '%s': %d project(s) added", msg.Name, msg.Assigned)
}

// reconcileRegularContextScope is wider than reapplyContextScope because editing
// one label moves projects between labels: a project removed from the edited
// label lands in another one, so any regular scope may have gained or lost
// members even when it was not the scope being edited. All resolves
// independently of labels and is left alone. Applying the scope also degrades
// it to All when the label has no members left, since applyContextScope
// re-resolves through the shared selectability boundary.
func (m *AppModel) reconcileRegularContextScope(wasActive bool, scope ContextScope) tea.Cmd {
	if m.crossOutliner == nil {
		return nil
	}
	if !wasActive {
		if m.crossOutliner.activeContext.Kind != ContextScopeRegular {
			return nil
		}
		scope = m.crossOutliner.activeContext
	}
	m.crossOutliner.applyContextScope(scope)
	return m.requestCrossProjectOutlinerReload()
}
