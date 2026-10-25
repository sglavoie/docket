package projectops

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
)

// projectOperationApplicability composes the entity-shape check with the same
// lifecycle and protection requirements execution enforces, resolved from the
// freshly loaded project rather than from the metadata a row was built with. A
// host therefore advertises only actions that can succeed, and a request forced
// against a project whose state changed is refused before the executor's
// destructive confirmation instead of after it. An unresolved project stays
// applicable so the handler reports the single authoritative "no longer
// available" error, and so does an operation that owns its own refusal.
func (c *Controller) projectOperationApplicability(
	shell Shell,
	id operation.OperationID,
	label string,
) operation.OperationApplicability {
	requirement, gated := projecthub.ProjectOperationRequirements[id]
	return func(ctx operation.CommandContext) bool {
		if !projecthub.ProjectOperationApplicable(ctx) {
			return false
		}
		if !outlinerProjectOperationTargetsHeader(id, ctx) {
			return false
		}
		project := shell.ProjectByID(operation.CurrentEntityIdentity(ctx).ProjectID)
		if project == nil {
			return true
		}
		if !gated || projectOperationsOwningTheirRefusal[id] {
			return true
		}
		return c.projectRequirementUnmet(shell, requirement, label, project) == nil
	}
}

// projectOperationsOwningTheirRefusal are the project operations whose handler
// reports an unmet requirement itself. The requirement is still enforced, at
// execution by projectOperationUnavailable; it is deliberately not mirrored into
// applicability, because applicability is checked again on the resumed
// submission and would refuse there with the executor's generic sentence,
// discarding text the user had already typed into the value step. Nothing is
// advertised dishonestly by the omission: the quick project comment is reached
// only from an Outliner project header, and that population is never archived,
// so the requirement can fail only after the prompt is already open.
var projectOperationsOwningTheirRefusal = map[operation.OperationID]bool{
	operation.OperationProjectAddComment: true,
}

// outlinerProjectHeaderOperations are the project operations an Outliner root
// key reaches, and therefore the ones the cursor alone has to identify. Every
// other project operation is entered from a sheet or a Hub row that already
// named its project, so it keeps ProjectOperationApplicable's sole-scope
// fallback unchanged.
var outlinerProjectHeaderOperations = map[operation.OperationID]bool{
	operation.OperationScopeRenameProject: true,
	operation.OperationProjectAddComment:  true,
	operation.OperationProjectComments:    true,
}

// outlinerProjectOperationTargetsHeader narrows the root-key project operations
// to an actual Outliner project header. `e`, `C`, and `;` are root keys there, so the
// cursor is the only thing that says which project the user meant:
// ProjectOperationApplicable's sole-scope fallback is right for a Hub row that
// named its project and wrong for a key press from a section header, a checklist
// row, or an empty projection, where it would silently edit, comment on, or open
// the comments of the one scoped project. The narrowing belongs to those three
// alone — the Hub keeps the fallback, and every other project operation keeps the
// applicability it had.
func outlinerProjectOperationTargetsHeader(id operation.OperationID, ctx operation.CommandContext) bool {
	if !outlinerProjectHeaderOperations[id] || !operation.CoreOutlinerApplicability(ctx) {
		return true
	}
	return ctx.Entity.HasProjectHeaderCursor &&
		!ctx.Entity.HasChecklistCursor &&
		!ctx.Selection.Visual
}

// RegisterProjectOperations binds every project operation to one handler.
// Archiving a project delegates confirmation because the shell's project
// archive prompt is also where the sandbox and system Inbox restrictions live;
// archiving completed tasks had no prompt at all and therefore uses the
// executor's canonical destructive confirmation. Deleting delegates for the
// same reason and one more: its prompt names the cascade the executor's generic
// sentence cannot count.
func (c *Controller) RegisterProjectOperations(shell Shell) error {
	if shell.Registry() == nil || shell.Executor() == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	for _, descriptor := range projecthub.ProjectOperationDescriptors() {
		descriptor.Applicability = c.projectOperationApplicability(shell, descriptor.ID, descriptor.Label)
		if err := shell.RegisterOperation(descriptor, operation.OperationBinding{
			SingleHandler: func(invocation operation.OperationInvocation, target operation.OperationTarget) operation.OperationHandlerResult {
				return c.ExecuteProjectOperation(shell, invocation, target)
			},
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

func (c *Controller) ExecuteProjectOperation(
	shell Shell,
	invocation operation.OperationInvocation,
	target operation.OperationTarget,
) operation.OperationHandlerResult {
	if target.Entity.Kind != operation.EntityKindProject || target.ProjectID == "" {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("%s needs a project target, got %s", invocation.Descriptor.Label, target.Entity),
		}
	}
	project := shell.ProjectByID(target.ProjectID)
	var unavailable error
	if project == nil {
		unavailable = fmt.Errorf("project %s is no longer available", target.ProjectID)
	} else {
		unavailable = c.projectOperationUnavailable(shell, invocation.Descriptor, project)
	}
	// The quick comment answers a refused owner itself rather than failing at the
	// shared gate, because it is the one project operation that may already be
	// holding text the user typed. Every other operation reaches the gate below
	// with nothing to lose.
	if invocation.Descriptor.ID == operation.OperationProjectAddComment {
		return projecthub.ProjectCommentOperationResult(invocation, project, unavailable)
	}
	if unavailable != nil {
		return operation.OperationHandlerResult{Err: unavailable}
	}
	if invocation.Descriptor.ID == operation.OperationProjectSetTags {
		return projecthub.ProjectTagsOperationResult(invocation, project)
	}
	message := c.projectOperationMessage(invocation.Descriptor.ID, project)
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
func (c *Controller) projectOperationUnavailable(
	shell Shell,
	descriptor operation.OperationDescriptor,
	project *models.Project,
) error {
	return c.projectRequirementUnmet(shell,
		projecthub.ProjectOperationRequirements[descriptor.ID],
		descriptor.Label,
		project,
	)
}

// projectRequirementUnmet names why one project fails a requirement. It is the
// single decision both advertisement and execution read, so a sheet can never
// offer an action the handler would refuse.
func (c *Controller) projectRequirementUnmet(
	shell Shell,
	requirement projecthub.ProjectOperationRequirement,
	label string,
	project *models.Project,
) error {
	switch {
	case requirement.RequireArchived && !project.Archived:
		return fmt.Errorf("%s applies only to archived projects", label)
	case requirement.RequireActive && project.Archived:
		return fmt.Errorf("%s is unavailable while %s is archived", label, project.Name)
	case requirement.RejectSystemUnfiled && shell.IsSystemUnfiledProject(project.ID):
		return fmt.Errorf("%s is unavailable for the system Unfiled project", label)
	}
	return nil
}

//nolint:exhaustive // Only project operations are registered with this handler.
func (c *Controller) projectOperationMessage(id operation.OperationID, project *models.Project) tea.Msg {
	switch id {
	case operation.OperationProjectOpen:
		return outliner.ApplyProjectScopeMsg{ProjectID: project.ID}
	case operation.OperationScopeRenameProject:
		return projecthub.EditProjectMsg{Project: project}
	case operation.OperationScopePinProject:
		return projecthub.ProjectPinRequestMsg{Project: project, Pinned: !project.Pinned}
	case operation.OperationScopeArchiveProject:
		return projecthub.ProjectArchiveRequestMsg{Project: project, Archive: true}
	case operation.OperationScopeRestoreProject:
		return projecthub.ProjectArchiveRequestMsg{Project: project, Archive: false}
	case operation.OperationProjectDelete:
		return projecthub.ProjectDeleteRequestMsg{Project: project}
	case operation.OperationScopeOpenProjectNote:
		return notes.ShowNotesManagerMsg{Scope: route.ProjectNotesScope(project.ID)}
	case operation.OperationProjectComments:
		return taskdetail.ShowProjectCommentsMsg{Project: project}
	default:
		return nil
	}
}
