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
		return true, m.activateManageHub(ManageParams{Module: ManageContexts, Scope: AppEntityScope()})

	case scopeHubInlineActionMsg:
		return true, m.openScopeHubForInlineAction(msg)

	case ContextScopeSelectedMsg:
		if msg.Canceled {
			return true, m.popView()
		}
		scope := RegularContextScope(msg.SelectedContext)
		if msg.IsCustomContext {
			scope = CustomContextScope(msg.SelectedContext)
		}
		m.recordFinderDestination(recentDestinationForContextScope(scope))
		if m.crossOutliner != nil {
			m.crossOutliner.applyContextScope(scope)
		}
		// Applying a scope is navigation to the scoped Outliner, not merely a
		// dismissal of the Context Hub. In particular, the global palette command
		// can open the hub above Task Detail; one pop would reveal that detail
		// again and could leave the user looking at a task outside the new scope.
		return true, tea.Batch(m.returnToOutlinerAfterContextSelection(), m.requestCrossProjectOutlinerReload())

	case CustomContextSavedMsg:
		// The hub committed the save through ContextMutator; the shell only
		// reconciles the Outliner scope and reports the outcome.
		verb := "Updated"
		switch {
		case msg.IsNew:
			verb = "Created"
		case msg.Renamed:
			verb = "Renamed"
		}
		m.setSuccessStatus(fmt.Sprintf("%s custom context '%s' with %d projects", verb, msg.Name, msg.ProjectCount))
		return true, tea.Batch(defaultStatusTimeout(), m.reapplyContextScope(msg.WasActive, CustomContextScope(msg.Name)))

	case CustomContextDeletedMsg:
		m.setSuccessStatus("Deleted custom context: " + msg.Name)
		return true, tea.Batch(defaultStatusTimeout(), m.reapplyContextScope(msg.WasActive, AllContextScope()))

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
// scope action started somewhere else, such as a Finder scope row.
func (m *AppModel) openScopeHubForInlineAction(msg scopeHubInlineActionMsg) tea.Cmd {
	cmd := m.activateManageHub(ManageParams{Module: ManageContexts, Scope: AppEntityScope()})
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
// Cancel uses the ordinary one-pop path instead so it can restore Task Detail.
func (m *AppModel) returnToOutlinerAfterContextSelection() tea.Cmd {
	m.navigation.ResetRoot()
	if view := m.views.Get(m.currentView()); view != nil {
		m.setAccentOnView(view)
	}
	return func() tea.Msg { return ViewDidAppearMsg{} }
}

func regularContextSavedStatus(msg RegularContextSavedMsg) string {
	if msg.Renamed {
		if msg.DefaultContextMoved {
			return fmt.Sprintf("Renamed default context '%s' to '%s' (%d project(s) relabeled)",
				msg.PreviousName, msg.Name, msg.RelabeledCount)
		}
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

// reapplyContextScope re-derives the Outliner scope after a mutation that
// touched the context it is scoped to: a rename follows the new name, a delete
// falls back to All, and a membership edit re-resolves the same scope against
// the new member list.
func (m *AppModel) reapplyContextScope(wasActive bool, scope ContextScope) tea.Cmd {
	if !wasActive || m.crossOutliner == nil {
		return nil
	}
	m.crossOutliner.applyContextScope(scope)
	return m.requestCrossProjectOutlinerReload()
}

// reconcileRegularContextScope is wider than reapplyContextScope because editing
// one label moves projects between labels: a project removed from the edited
// label lands in another one, so any regular scope may have gained or lost
// members even when it was not the scope being edited. All and custom scopes
// resolve independently of labels and are left alone. Applying the scope also
// degrades it to All when the label has no members left, since applyContextScope
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
