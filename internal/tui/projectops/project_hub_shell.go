package projectops

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

func (c *Controller) RegisterProjectHubOperations(shell Shell) error {
	for _, descriptor := range projecthub.ProjectHubOperationDescriptors() {
		if err := shell.RegisterOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityProjectHub),
			Handler: func(invocation operation.OperationInvocation) operation.OperationHandlerResult {
				return c.executeProjectHubOperation(shell, invocation)
			},
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
func (c *Controller) executeProjectHubOperation(shell Shell, invocation operation.OperationInvocation) operation.OperationHandlerResult {
	if invocation.Descriptor.ID == operation.OperationScopeCreateProject {
		return operation.OperationHandlerResult{Message: projecthub.CreateProjectMsg{}}
	}
	hub := c.activeProjectHub(shell)
	if hub == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Project Hub is not open")}
	}
	switch invocation.Descriptor.ID {
	case projecthub.OperationProjectHubSearch:
		return operation.OperationHandlerResult{Message: operation.CommandMessage(hub.ActivateSearch())}
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
	case projecthub.OperationProjectHubSplitLeft:
		hub.ShiftPreviewSplit(-1)
	case projecthub.OperationProjectHubSplitRight:
		hub.ShiftPreviewSplit(1)
	}
	return operation.OperationHandlerResult{}
}

// activeProjectHub returns the mounted Project Hub, or nil when Manage is on
// another module.
func (c *Controller) activeProjectHub(shell Shell) *projecthub.ProjectHubModel {
	if shell == nil || shell.ManageHub() == nil || shell.ManageHub().Params().Module != route.ManageProjects {
		return nil
	}
	hub, _ := shell.ManageHub().ActiveModuleView().(*projecthub.ProjectHubModel)
	return hub
}

func (c *Controller) RefreshProjectHubIfActive(shell Shell) {
	hub := c.activeProjectHub(shell)
	if hub == nil {
		return
	}
	hub.RefreshProjectHub()
	hub.SetTaskCounts(c.ProjectHubTaskCounts(shell))
	if shell.Outliner() != nil {
		hub.SetProjectFocus(shell.Outliner().Scope.ProjectID)
	}
}

// SeatProjectHubCursor lands an open Hub's cursor on one project's row.
func (c *Controller) SeatProjectHubCursor(shell Shell, projectID string) {
	if hub := c.activeProjectHub(shell); hub != nil {
		hub.SeatCursorOnProject(projectID)
	}
}

// ProjectHubTaskCounts counts tasks for every cataloged project, not just the
// base workspace the Outliner loads, so the Hub can report parked and archived
// rows on a real number instead of an unknown. The projects the Outliner already
// parsed seed the map for free; the remainder is loaded through the Outliner's
// own retained loader, which serves an unchanged project from its cache rather
// than reparsing it on every Hub open.
//
// A project that fails to load is absent from the map rather than counted zero,
// and its diagnostic is dropped: the Hub is a picker, and a project it cannot
// parse still needs to be listed and selectable.
func (c *Controller) ProjectHubTaskCounts(shell Shell) map[string]projecthub.ProjectTaskTally {
	counts := make(map[string]projecthub.ProjectTaskTally)
	if shell == nil {
		return counts
	}
	var loader *workspace.Loader
	if shell.Outliner() != nil {
		for _, project := range shell.Outliner().Population.Projects {
			if project != nil {
				counts[project.ID] = projecthub.ProjectTaskTallyOf(project)
			}
		}
		if shell.Outliner().ProjectService != nil {
			loader = shell.Outliner().ProjectService.WorkspaceLoader()
		}
	}

	seen := make(map[string]bool, len(counts))
	for id := range counts {
		seen[id] = true
	}
	cataloged := append(shell.AvailableProjects(), shell.ArchivedProjects()...)
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
