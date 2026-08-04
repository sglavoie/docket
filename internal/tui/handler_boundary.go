package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Boundary mutations all share one shape: clone the runtime domains, apply the
// defensive catalog operation to the clone, commit the clone through one checked
// workspace transaction, and only then publish catalog metadata, rebuild the
// caches, and report success. A validation rejection or a failed commit
// abandons the clone, so neither the runtime snapshot nor any live view model
// can observe speculative boundary state.

// handleBoundaryMessages applies the four boundary mutations. Returns true if
// the message was handled.
func (m *AppModel) handleBoundaryMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case CreateBoundaryMsg:
		return true, m.commitBoundaryCreate(msg.Name)

	case RenameBoundaryMsg:
		return true, m.commitBoundaryRename(msg.BoundaryID, msg.Name)

	case DeleteBoundaryMsg:
		return true, m.commitBoundaryDelete(msg.BoundaryID, msg.DestinationID)

	case AssignProjectBoundaryMsg:
		return true, m.commitProjectBoundaryAssignment(msg.ProjectID, msg.BoundaryID)
	}
	return false, nil
}

func (m *AppModel) commitBoundaryCreate(name string) tea.Cmd {
	domains := cloneRuntimeDomains(m.runtime)
	if domains == nil {
		return errorCmd("cannot create a boundary: runtime state is unavailable")
	}
	created, err := domains.Catalog.CreateBoundary(name)
	if err != nil {
		return errorCmd("%v", err)
	}
	if err := m.commitBoundaryDomains(domains, false, "Create Boundary", created.Name, nil); err != nil {
		return errorCmd("%v", err)
	}
	m.setSuccessStatus(fmt.Sprintf("Created boundary '%s'", created.Name))
	return defaultStatusTimeout()
}

func (m *AppModel) commitBoundaryRename(boundaryID, name string) tea.Cmd {
	domains := cloneRuntimeDomains(m.runtime)
	if domains == nil {
		return errorCmd("cannot rename a boundary: runtime state is unavailable")
	}
	previous, _ := domains.Catalog.Boundary(boundaryID)
	if err := domains.Catalog.RenameBoundary(boundaryID, name); err != nil {
		return errorCmd("%v", err)
	}
	renamed, _ := domains.Catalog.Boundary(boundaryID)
	if err := m.commitBoundaryDomains(domains, false, "Rename Boundary", renamed.Name, nil); err != nil {
		return errorCmd("%v", err)
	}
	m.setSuccessStatus(fmt.Sprintf("Renamed boundary '%s' to '%s'", previous.Name, renamed.Name))
	return defaultStatusTimeout()
}

func (m *AppModel) commitBoundaryDelete(boundaryID, destinationID string) tea.Cmd {
	domains := cloneRuntimeDomains(m.runtime)
	if domains == nil {
		return errorCmd("cannot delete a boundary: runtime state is unavailable")
	}
	deleted, _ := domains.Catalog.Boundary(boundaryID)
	destination, _ := domains.Catalog.Boundary(destinationID)
	moved, err := domains.Catalog.DeleteBoundary(boundaryID, destinationID)
	if err != nil {
		return errorCmd("%v", err)
	}
	// A boundary that can be deleted is never Active, so its members always move
	// between boundaries the task workspace already excluded or already included
	// as a group; no planning membership can be stranded by the move.
	if err := m.commitBoundaryDomains(domains, false, "Delete Boundary", deleted.Name, moved); err != nil {
		return errorCmd("%v", err)
	}
	m.setSuccessStatus(fmt.Sprintf("Deleted boundary '%s': %d project(s) moved to '%s'",
		deleted.Name, len(moved), destination.Name))
	return m.boundaryMembershipRefresh()
}

func (m *AppModel) commitProjectBoundaryAssignment(projectID, boundaryID string) tea.Cmd {
	domains := cloneRuntimeDomains(m.runtime)
	if domains == nil {
		return errorCmd("cannot set a project boundary: runtime state is unavailable")
	}
	project, ok := domains.Catalog.CatalogProjectByID(projectID)
	if !ok {
		return errorCmd("cannot set a project boundary: project %q is not in the catalog", projectID)
	}
	previous := domains.Catalog.ProjectBoundaryID(projectID)
	changed, err := domains.Catalog.AssignProjectBoundary(projectID, boundaryID)
	if err != nil {
		return errorCmd("%v", err)
	}
	boundary, _ := domains.Catalog.Boundary(boundaryID)
	if !changed {
		m.setInfoStatus(fmt.Sprintf("%s is already in '%s'", project.Name, boundary.Name))
		return defaultStatusTimeout()
	}

	// Leaving Active is the only transition that can strand planning
	// memberships, and an archived project's were already cleared when it was
	// archived. Every other assignment is a catalog-only change.
	planningCleared := false
	if previous == models.ActiveBoundaryID && boundaryID != models.ActiveBoundaryID && !project.Archived {
		planningCleared = domains.Planning.RemoveProjectReferences(project.ID, project.Slug)
	}
	if err := m.commitBoundaryDomains(
		domains, planningCleared, "Set Project Boundary", project.Name, []string{projectID},
	); err != nil {
		return errorCmd("%v", err)
	}
	if planningCleared {
		m.rebuildPlanningFromRuntime()
	}
	m.setSuccessStatus(fmt.Sprintf("Moved %s to '%s'", project.Name, boundary.Name))
	return m.boundaryMembershipRefresh()
}

// commitBoundaryDomains commits a prepared boundary mutation as one checked,
// undoable workspace transaction. Planning joins the write set only when the
// mutation actually cleared memberships, so a catalog-only boundary change
// cannot fail on a planning revision it never intended to advance. Machine-local
// state is never part of a boundary change and stays out of the transaction.
//
// Every published mutation resyncs an open Project Hub, because the Hub groups
// and filters by boundary: a created, renamed, or deleted boundary changes its
// listing even when no project moved.
func (m *AppModel) commitBoundaryDomains(
	domains *config.Domains,
	planningCleared bool,
	action string,
	undoContext string,
	projectIDs []string,
) error {
	if m.runtime.DataDir() == "" || m.appService == nil || m.configManager == nil {
		m.runtime.SetDomainsInMemory(domains)
		m.syncProjectCache()
		m.refreshProjectHubIfActive()
		return nil
	}
	var artifacts []storage.ArtifactChange
	var err error
	if planningCleared {
		artifacts, err = syncedDomainArtifacts(m.runtime, domains.Catalog, domains.Planning)
	} else {
		artifacts, err = catalogDomainArtifact(m.runtime, domains.Catalog)
	}
	if err != nil {
		return err
	}
	if err = m.appService.CommitProjectArtifacts(storage.ArtifactCommitRequest{
		Artifacts:   artifacts,
		ProjectIDs:  projectIDs,
		UndoAction:  action,
		UndoContext: undoContext,
	}); err != nil {
		return err
	}
	if err = m.runtime.RefreshAfterCommit(); err != nil {
		return err
	}
	m.syncProjectCache()
	m.refreshProjectHubIfActive()
	return nil
}

// boundaryMembershipRefresh rebuilds the surfaces that read project membership
// after a boundary mutation moved at least one project.
func (m *AppModel) boundaryMembershipRefresh() tea.Cmd {
	m.refreshScopeHubIfActive()
	return tea.Batch(
		m.requestCrossProjectOutlinerReload(),
		m.refreshActiveNotesManager(),
		defaultStatusTimeout(),
	)
}
