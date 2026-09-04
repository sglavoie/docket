package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// projectOperationApplicability composes the entity-shape check with the same
// lifecycle and protection requirements execution enforces, resolved from the
// freshly loaded project rather than from the metadata a row was built with. A
// host therefore advertises only actions that can succeed, and a request forced
// against a project whose state changed is refused before the executor's
// destructive confirmation instead of after it. An unresolved project stays
// applicable so the handler reports the single authoritative "no longer
// available" error.
func (m *AppModel) projectOperationApplicability(
	id operation.OperationID,
	label string,
) operation.OperationApplicability {
	requirement, gated := projectOperationRequirements[id]
	return func(ctx operation.CommandContext) bool {
		if !projectOperationApplicable(ctx) {
			return false
		}
		project := m.projectByID(operation.CurrentEntityIdentity(ctx).ProjectID)
		if project == nil {
			return true
		}
		if !gated {
			return true
		}
		return m.projectRequirementUnmet(requirement, label, project) == nil
	}
}

// registerProjectOperations binds every project operation to one handler.
// Archiving a project delegates confirmation because the shell's project
// archive prompt is also where the sandbox and system Inbox restrictions live;
// archiving completed tasks had no prompt at all and therefore uses the
// executor's canonical destructive confirmation. Deleting delegates for the
// same reason and one more: its prompt names the cascade the executor's generic
// sentence cannot count.
func (m *AppModel) registerProjectOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	for _, descriptor := range projectOperationDescriptors() {
		descriptor.Applicability = m.projectOperationApplicability(descriptor.ID, descriptor.Label)
		if err := m.registerOperation(descriptor, operation.OperationBinding{
			SingleHandler:         m.executeProjectOperation,
			DelegateValueProvider: true,
			DelegateTransaction:   true,
			DelegateConfirmation: descriptor.ID == operation.OperationScopeArchiveProject ||
				descriptor.ID == operation.OperationProjectDelete,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (m *AppModel) executeProjectOperation(
	invocation operation.OperationInvocation,
	target operation.OperationTarget,
) operation.OperationHandlerResult {
	if target.Entity.Kind != operation.EntityKindProject || target.ProjectID == "" {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("%s needs a project target, got %s", invocation.Descriptor.Label, target.Entity),
		}
	}
	project := m.projectByID(target.ProjectID)
	if project == nil {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("project %s is no longer available", target.ProjectID),
		}
	}
	if err := m.projectOperationUnavailable(invocation.Descriptor, project); err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if invocation.Descriptor.ID == operation.OperationProjectSetTags {
		return projectTagsOperationResult(invocation, project)
	}
	message := m.projectOperationMessage(invocation.Descriptor.ID, project)
	if message == nil {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("project operation %s has no action", invocation.Descriptor.ID),
		}
	}
	return operation.OperationHandlerResult{Message: message}
}

// projectOperationUnavailable checks the freshly reloaded project rather than
// the state a row was built from, so a project archived or removed since the
// action sheet opened fails loudly instead of acting on stale metadata.
func (m *AppModel) projectOperationUnavailable(
	descriptor operation.OperationDescriptor,
	project *models.Project,
) error {
	return m.projectRequirementUnmet(
		projectOperationRequirements[descriptor.ID],
		descriptor.Label,
		project,
	)
}

// projectRequirementUnmet names why one project fails a requirement. It is the
// single decision both advertisement and execution read, so a sheet can never
// offer an action the handler would refuse.
func (m *AppModel) projectRequirementUnmet(
	requirement projectOperationRequirement,
	label string,
	project *models.Project,
) error {
	switch {
	case requirement.requireArchived && !project.Archived:
		return fmt.Errorf("%s applies only to archived projects", label)
	case requirement.requireActive && project.Archived:
		return fmt.Errorf("%s is unavailable while %s is archived", label, project.Name)
	case requirement.rejectSystemUnfiled && m.runtime.IsSystemUnfiledProject(project.ID):
		return fmt.Errorf("%s is unavailable for the system Unfiled project", label)
	}
	return nil
}

//nolint:exhaustive // Only project operations are registered with this handler.
func (m *AppModel) projectOperationMessage(id operation.OperationID, project *models.Project) tea.Msg {
	switch id {
	case operation.OperationProjectOpen:
		return ApplyProjectScopeMsg{ProjectID: project.ID}
	case operation.OperationScopeRenameProject:
		return EditProjectMsg{Project: project}
	case operation.OperationScopePinProject:
		return ProjectPinRequestMsg{Project: project, Pinned: !project.Pinned}
	case operation.OperationScopeArchiveProject:
		return ProjectArchiveRequestMsg{Project: project, Archive: true}
	case operation.OperationScopeRestoreProject:
		return ProjectArchiveRequestMsg{Project: project, Archive: false}
	case operation.OperationProjectDelete:
		return ProjectDeleteRequestMsg{Project: project}
	case operation.OperationScopeOpenProjectNote:
		return ShowNotesManagerMsg{Scope: route.ProjectNotesScope(project.ID)}
	default:
		return nil
	}
}
