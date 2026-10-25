package projecthub

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// ProjectOperationDescriptors is the authoritative catalog of project-targeted
// operations. Every entry resolves one frozen project identity, so the same
// descriptor serves the Scope Hub and any other host that names a
// project row.
func ProjectOperationDescriptors() []operation.OperationDescriptor {
	descriptor := func(
		id operation.OperationID,
		label string,
		description string,
		category operation.OperationCategory,
		risk operation.OperationRisk,
		aliases ...string,
	) operation.OperationDescriptor {
		return operation.OperationDescriptor{
			ID:                id,
			Label:             label,
			Description:       description,
			Aliases:           aliases,
			Category:          category,
			TargetSource:      operation.OperationTargetCurrentEntity,
			TargetCardinality: operation.TargetCardinalityOne,
			Applicability:     ProjectOperationApplicable,
			Risk:              risk,
		}
	}
	return []operation.OperationDescriptor{
		descriptor(operation.OperationProjectOpen, "Open Project", "Focus the Outliner on this project", operation.OperationCategoryProject, operation.OperationRiskReadOnly, "focus project"),
		descriptor(operation.OperationScopeRenameProject, "Edit Project", "Edit lean project metadata", operation.OperationCategoryProject, operation.OperationRiskMutation, "project settings"),
		descriptor(operation.OperationScopePinProject, "Toggle Project Pin", "Pin or unpin the project", operation.OperationCategoryProject, operation.OperationRiskMutation, "pin project", "unpin project"),
		descriptor(operation.OperationScopeArchiveProject, "Archive Project", "Archive the project", operation.OperationCategoryProject, operation.OperationRiskDestructive),
		descriptor(operation.OperationScopeRestoreProject, "Unarchive Project", "Return the project to active", operation.OperationCategoryProject, operation.OperationRiskMutation, "restore archived project"),
		descriptor(operation.OperationProjectDelete, "Delete Project", "Delete the project and everything it owns", operation.OperationCategoryProject, operation.OperationRiskDestructive, "remove project", "destroy project"),
		descriptor(operation.OperationScopeOpenProjectNote, "Open Project Notes", "Open notes owned by this project", operation.OperationCategoryNote, operation.OperationRiskReadOnly),
		operation.WithValueProvider(
			descriptor(operation.OperationProjectSetTags, "Set Project Tags", "Edit the tags carried by this project", operation.OperationCategoryProject, operation.OperationRiskMutation, "project tags", "park project", "unpark project"),
			operation.ValueProviderProjectTags,
		),
		descriptor(operation.OperationProjectComments, "Project Comments", "Read, edit, and delete this project's comments", operation.OperationCategoryProject, operation.OperationRiskReadOnly, "project comment panel", "read project comments"),
		operation.WithValueProvider(
			descriptor(operation.OperationProjectAddComment, "Add Project Comment", "Append one comment to this project", operation.OperationCategoryProject, operation.OperationRiskMutation, "comment on project", "project comment"),
			operation.ValueProviderProjectComment,
		),
	}
}

// ProjectOperationApplicable admits a project operation only when the context
// names exactly one project. CurrentEntityIdentity degrades to the first scoped
// project when nothing is focused, so a multi-project scope is refused rather
// than allowed to borrow an unrelated project.
func ProjectOperationApplicable(ctx operation.CommandContext) bool {
	// A section header names a lane, not a project. The scope fallback below would
	// hand it the sole scoped project, which is how a key meant for a project
	// header reaches one from a row that never named it.
	if ctx.Entity.HasSectionHeaderCursor {
		return false
	}
	if operation.CurrentEntityIdentity(ctx).Kind != operation.EntityKindProject {
		return false
	}
	if operation.NormalizeEntityIdentity(ctx.CurrentEntity).Kind == operation.EntityKindProject || ctx.Entity.ProjectID != "" {
		return true
	}
	return len(operation.NormalizedProjectIDs(ctx.ScopeProjectIDs)) == 1
}

// ProjectTagsOperationResult round-trips the project's own tag set through the
// value picker: the first invocation asks for a set, the resumed one carries it.
// The chosen set replaces the stored set wholesale, so an empty slice is a
// legitimate value and only a missing one reopens the picker.
func ProjectTagsOperationResult(
	invocation operation.OperationInvocation,
	project *models.Project,
) operation.OperationHandlerResult {
	tags, ok := invocation.Value.([]string)
	if !invocation.HasValue || !ok {
		return operation.OperationHandlerResult{Message: operation.OperationValueRequestMsg{
			Request: operation.OperationExecutionRequest{
				ID:      invocation.Descriptor.ID,
				Context: operation.CloneCommandContext(invocation.Context),
			},
			Provider: operation.ValueProviderProjectTags,
		}}
	}
	return operation.OperationHandlerResult{Message: SetProjectTagsMsg{Project: project, Tags: tags}}
}

// ProjectCommentOperationResult round-trips one typed comment through the value
// prompt and then hands the commit to the shell.
//
// The unavailable argument is the reason this owner cannot be written to, or nil
// when it can. It is threaded in rather than checked here because the answer
// depends on the freshly reloaded project, and because what it means differs
// with what the user has already invested: before anything is typed a refused
// owner is an ordinary error, while after a paragraph is typed the same refusal
// must reopen the buffer instead of discarding it. That is the whole reason this
// operation does not simply fail at the shared availability gate.
func ProjectCommentOperationResult(
	invocation operation.OperationInvocation,
	project *models.Project,
	unavailable error,
) operation.OperationHandlerResult {
	request := operation.OperationExecutionRequest{
		ID:      invocation.Descriptor.ID,
		Context: operation.CloneCommandContext(invocation.Context),
	}
	text, ok := invocation.Value.(string)
	if !invocation.HasValue || !ok {
		if unavailable != nil {
			return operation.OperationHandlerResult{Err: unavailable}
		}
		return operation.OperationHandlerResult{Message: operation.OperationValueRequestMsg{
			Request:  request,
			Provider: operation.ValueProviderProjectComment,
		}}
	}
	if unavailable != nil {
		return operation.OperationHandlerResult{Message: ProjectCommentRejectedMsg{
			Request: request,
			Text:    text,
			Err:     unavailable,
		}}
	}
	return operation.OperationHandlerResult{Message: AddProjectCommentMsg{
		Request: request,
		Project: project,
		Text:    text,
	}}
}

// ProjectOperationRequirement declares the project state one operation needs.
type ProjectOperationRequirement struct {
	RequireArchived     bool
	RequireActive       bool
	RejectSystemUnfiled bool
}

var ProjectOperationRequirements = map[operation.OperationID]ProjectOperationRequirement{
	operation.OperationProjectOpen:          {RequireActive: true},
	operation.OperationScopePinProject:      {RequireActive: true},
	operation.OperationScopeArchiveProject:  {RequireActive: true, RejectSystemUnfiled: true},
	operation.OperationScopeRestoreProject:  {RequireArchived: true},
	operation.OperationScopeOpenProjectNote: {RequireActive: true},
	operation.OperationProjectSetTags:       {RequireActive: true},
	// Parked and Unfiled are deliberately absent from this requirement: parking
	// hides a project rather than sealing it, and Unfiled is protected against
	// lifecycle changes, not against content.
	operation.OperationProjectAddComment: {RequireActive: true},
	// Opening the panel carries the same lifecycle requirement the append does, so
	// the two halves of `C`/`;` cannot disagree about which projects accept
	// comments. An archived project's comments are read through the document, not
	// through a surface whose every mutation would be refused.
	operation.OperationProjectComments: {RequireActive: true},
	// Deleting carries no lifecycle requirement on purpose: an archived project
	// stays deletable, so archiving is a step toward removal rather than a state
	// that traps the project in the catalog.
	operation.OperationProjectDelete: {RejectSystemUnfiled: true},
}
