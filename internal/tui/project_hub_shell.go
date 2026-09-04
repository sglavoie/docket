package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/workspace"
)

// The shell half of the Project Hub: the registrar, the handler the registry
// binds, and the two reads that need the live app -- the mounted module and the
// cross-scope loader the per-project counts come from. The surface itself is
// internal/tui/projecthub; only the parts that name AppModel are here.

func (m *AppModel) registerProjectHubOperations() error {
	for _, descriptor := range projecthub.ProjectHubOperationDescriptors() {
		if err := m.registerOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities:     operation.NewCapabilitySet(operation.CapabilityProjectHub),
			Handler:                  m.executeProjectHubOperation,
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// executeProjectHubOperation applies a Hub surface operation to the live Hub.
// The list vocabulary mutates the model the user is looking at, exactly as the
// Outliner's own core operations do, so the handler reaches for the mounted
// module rather than rebuilding one.
//
//nolint:exhaustive // Only Project Hub surface operations are registered with this handler.
func (m *AppModel) executeProjectHubOperation(invocation operation.OperationInvocation) operation.OperationHandlerResult {
	if invocation.Descriptor.ID == operation.OperationScopeCreateProject {
		return operation.OperationHandlerResult{Message: projecthub.CreateProjectMsg{}}
	}
	hub := m.activeProjectHub()
	if hub == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Project Hub is not open")}
	}
	switch invocation.Descriptor.ID {
	case projecthub.OperationProjectHubSearch:
		return operation.OperationHandlerResult{Message: commandMessage(hub.ActivateSearch())}
	case projecthub.OperationProjectHubToggleSelection:
		hub.ToggleProjectHubSelection()
	case projecthub.OperationProjectHubSelectAll:
		hub.SelectEveryMatchingProject()
	case projecthub.OperationProjectHubActions:
		hub.OpenProjectHubActions()
	case projecthub.OperationProjectHubTaskFilter:
		hub.CycleProjectHubTaskFilter()
	case projecthub.OperationProjectHubPreviewPageDown:
		hub.PageProjectPreview(1)
	case projecthub.OperationProjectHubPreviewPageUp:
		hub.PageProjectPreview(-1)
	}
	return operation.OperationHandlerResult{}
}

// activeProjectHub returns the mounted Project Hub, or nil when Manage is on
// another module.
func (m *AppModel) activeProjectHub() *projecthub.ProjectHubModel {
	if m == nil || m.manageHub == nil || m.manageHub.Params().Module != route.ManageProjects {
		return nil
	}
	hub, _ := m.manageHub.ActiveModuleView().(*projecthub.ProjectHubModel)
	return hub
}

func (m *AppModel) refreshProjectHubIfActive() {
	hub := m.activeProjectHub()
	if hub == nil {
		return
	}
	hub.RefreshProjectHub()
	hub.SetTaskCounts(m.projectHubTaskCounts())
	if m.crossOutliner != nil {
		hub.SetProjectFocus(m.crossOutliner.Scope.ProjectID)
	}
}

// projectHubTaskCounts counts tasks for every cataloged project, not just the
// base workspace the Outliner loads, so the Hub can report parked and archived
// rows on a real number instead of an unknown. The projects the Outliner already
// parsed seed the map for free; the remainder is loaded through the Outliner's
// own retained loader, which serves an unchanged project from its cache rather
// than reparsing it on every Hub open.
//
// A project that fails to load is absent from the map rather than counted zero,
// and its diagnostic is dropped: the Hub is a picker, and a project it cannot
// parse still needs to be listed and selectable.
func (m *AppModel) projectHubTaskCounts() map[string]projecthub.ProjectTaskTally {
	counts := make(map[string]projecthub.ProjectTaskTally)
	if m == nil {
		return counts
	}
	var loader *workspace.Loader
	if m.crossOutliner != nil {
		for _, project := range m.crossOutliner.Population.Projects {
			if project != nil {
				counts[project.ID] = projecthub.ProjectTaskTallyOf(project)
			}
		}
		if m.crossOutliner.ProjectService != nil {
			loader = m.crossOutliner.ProjectService.WorkspaceLoader()
		}
	}
	if m.domain.runtime == nil {
		return counts
	}

	seen := make(map[string]bool, len(counts))
	for id := range counts {
		seen[id] = true
	}
	cataloged := append(m.domain.runtime.AvailableProjects(), m.domain.runtime.ArchivedProjects()...)
	remainder := make([]*models.Project, 0, len(cataloged))
	for _, project := range cataloged {
		if project == nil || seen[project.ID] {
			continue
		}
		seen[project.ID] = true
		remainder = append(remainder, project)
	}
	if len(remainder) == 0 {
		return counts
	}
	_, loaded, _ := outliner.LoadCrossScopeSelection(remainder, loader)
	for _, project := range loaded {
		if project != nil {
			counts[project.ID] = projecthub.ProjectTaskTallyOf(project)
		}
	}
	return counts
}
