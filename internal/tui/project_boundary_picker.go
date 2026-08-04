package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// The boundary assignment surface is deliberately its own path. It shares the
// generic ListSurface picker shell with the task providers — newOperationValuePickerModel,
// OperationValueSelectedMsg, and the overlay routing in openOperationValueStep —
// but nothing else: its subject is a project, so it resolves its own target
// before task-reference resolution and emits a durable boundary ID rather than
// any task-shaped value.

const projectBoundaryItemPrefix = "project-boundary:"

// projectBoundaryOperationResult turns one Set boundary invocation into either
// the assignment message or a request for the missing boundary. A host that
// froze its own row payload in Value has not chosen a boundary, so the operation
// asks for one and re-enters the executor with the answer, which revalidates the
// project against the catalog the choice was made from.
func projectBoundaryOperationResult(
	invocation OperationInvocation,
	project *models.Project,
) OperationHandlerResult {
	boundaryID, ok := invocation.Value.(string)
	if !invocation.HasValue || !ok || boundaryID == "" {
		return OperationHandlerResult{Message: OperationValueRequestMsg{
			Request: OperationExecutionRequest{
				ID:      invocation.Descriptor.ID,
				Context: cloneCommandContext(invocation.Context),
			},
			Provider: ValueProviderProjectBoundary,
		}}
	}
	return OperationHandlerResult{Message: AssignProjectBoundaryMsg{
		ProjectID:  project.ID,
		BoundaryID: boundaryID,
	}}
}

// newProjectBoundaryPicker lists every defined boundary for the project named by
// the frozen context, preselecting the one it currently belongs to. Creating a
// boundary stays exclusive to the Boundary Manager, so the picker offers only
// what the catalog already defines.
func (m *AppModel) newProjectBoundaryPicker(
	request OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	projectID := currentEntityIdentity(request.Context).ProjectID
	project := m.projectByID(projectID)
	if project == nil {
		return nil, fmt.Errorf("project %s is no longer available", projectID)
	}
	catalog := m.runtime.Catalog()
	boundaries := orderedProjectBoundaries(catalog)
	if len(boundaries) == 0 {
		return nil, fmt.Errorf("no boundaries are defined")
	}

	items := make([]ListItem, 0, len(boundaries))
	for _, boundary := range boundaries {
		items = append(items, ListItem{
			ID:            ListItemID(projectBoundaryItemPrefix + boundary.ID),
			PrimaryText:   boundary.Name,
			SecondaryText: projectBoundarySecondary(boundary.ID),
			Aliases:       []string{boundary.Name},
			Data:          boundary.ID,
		})
	}
	return newOperationValuePickerModel(
		request,
		ValueProviderProjectBoundary,
		"Set Boundary",
		project.Name,
		ListSource{Items: items},
		false,
		[]ListItemID{ListItemID(projectBoundaryItemPrefix + resolvedProjectBoundaryID(catalog, projectID))},
	)
}

// orderedProjectBoundaries lists Active first and every custom boundary after it
// in a stable case-insensitive order, so the same sequence backs the picker and
// the Project Editor selector.
func orderedProjectBoundaries(catalog *config.WorkspaceCatalog) []config.ProjectBoundary {
	if catalog == nil {
		return nil
	}
	boundaries := append([]config.ProjectBoundary(nil), catalog.Boundaries...)
	slices.SortFunc(boundaries, func(a, b config.ProjectBoundary) int {
		if a.ID == models.ActiveBoundaryID || b.ID == models.ActiveBoundaryID {
			if a.ID == b.ID {
				return 0
			}
			if a.ID == models.ActiveBoundaryID {
				return -1
			}
			return 1
		}
		if order := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return boundaries
}

// resolvedProjectBoundaryID reports the boundary a project is listed under. An
// unassigned project and one naming a boundary the catalog no longer defines
// both resolve to Active, so a preselection can never miss the list.
func resolvedProjectBoundaryID(catalog *config.WorkspaceCatalog, projectID string) string {
	if catalog == nil {
		return models.ActiveBoundaryID
	}
	boundaryID := catalog.ProjectBoundaryID(projectID)
	if _, ok := catalog.Boundary(boundaryID); !ok {
		return models.ActiveBoundaryID
	}
	return boundaryID
}

func projectBoundarySecondary(boundaryID string) string {
	if boundaryID == models.ActiveBoundaryID {
		return "in the task workspace"
	}
	return "outside the task workspace"
}
