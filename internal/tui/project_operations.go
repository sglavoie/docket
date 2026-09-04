package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// projectOperationDescriptors is the authoritative catalog of project-targeted
// operations. Every entry resolves one frozen project identity, so the same
// descriptor serves the Scope Hub and any other host that names a
// project row.
func projectOperationDescriptors() []operation.OperationDescriptor {
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
			Applicability:     projectOperationApplicable,
			Risk:              risk,
		}
	}
	return []operation.OperationDescriptor{
		descriptor(operation.OperationProjectOpen, "Open project", "Focus the Outliner on this project", operation.OperationCategoryProject, operation.OperationRiskReadOnly, "focus project"),
		descriptor(operation.OperationScopeRenameProject, "Edit project", "Edit lean project metadata", operation.OperationCategoryProject, operation.OperationRiskMutation, "project settings"),
		descriptor(operation.OperationScopePinProject, "Toggle project pin", "Pin or unpin the project", operation.OperationCategoryProject, operation.OperationRiskMutation, "pin project", "unpin project"),
		descriptor(operation.OperationScopeArchiveProject, "Archive project", "Archive the project", operation.OperationCategoryProject, operation.OperationRiskDestructive),
		descriptor(operation.OperationScopeRestoreProject, "Unarchive project", "Return the project to active", operation.OperationCategoryProject, operation.OperationRiskMutation, "restore archived project"),
		descriptor(operation.OperationProjectDelete, "Delete project", "Delete the project and everything it owns", operation.OperationCategoryProject, operation.OperationRiskDestructive, "remove project", "destroy project"),
		descriptor(operation.OperationScopeOpenProjectNote, "Open project notes", "Open notes owned by this project", operation.OperationCategoryNote, operation.OperationRiskReadOnly),
		operation.WithValueProvider(
			descriptor(operation.OperationProjectSetTags, "Set project tags", "Edit the tags carried by this project", operation.OperationCategoryProject, operation.OperationRiskMutation, "project tags", "park project", "unpark project"),
			operation.ValueProviderProjectTags,
		),
	}
}

// projectOperationApplicable admits a project operation only when the context
// names exactly one project. CurrentEntityIdentity degrades to the first scoped
// project when nothing is focused, so a multi-project scope is refused rather
// than allowed to borrow an unrelated project.
func projectOperationApplicable(ctx operation.CommandContext) bool {
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

// projectTagsOperationResult round-trips the project's own tag set through the
// value picker: the first invocation asks for a set, the resumed one carries it.
// The chosen set replaces the stored set wholesale, so an empty slice is a
// legitimate value and only a missing one reopens the picker.
func projectTagsOperationResult(
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

// projectOperationRequirement declares the project state one operation needs.
type projectOperationRequirement struct {
	requireArchived     bool
	requireActive       bool
	rejectSystemUnfiled bool
}

var projectOperationRequirements = map[operation.OperationID]projectOperationRequirement{
	operation.OperationProjectOpen:          {requireActive: true},
	operation.OperationScopePinProject:      {requireActive: true},
	operation.OperationScopeArchiveProject:  {requireActive: true, rejectSystemUnfiled: true},
	operation.OperationScopeRestoreProject:  {requireArchived: true},
	operation.OperationScopeOpenProjectNote: {requireActive: true},
	operation.OperationProjectSetTags:       {requireActive: true},
	// Deleting carries no lifecycle requirement on purpose: an archived project
	// stays deletable, so archiving is a step toward removal rather than a state
	// that traps the project in the catalog.
	operation.OperationProjectDelete: {rejectSystemUnfiled: true},
}
