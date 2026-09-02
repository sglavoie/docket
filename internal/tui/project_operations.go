package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// Project operation IDs. The tui.scope.* IDs predate this catalog and keep
// their identity because they already name the same action from the Scope Hub;
// tui.project.* is reserved for the maintenance actions added alongside them.
const (
	operationScopeRenameProject   OperationID = "tui.scope.rename-project"
	operationScopePinProject      OperationID = "tui.scope.pin-project"
	operationScopeArchiveProject  OperationID = "tui.scope.archive-project"
	operationScopeRestoreProject  OperationID = "tui.scope.restore-project"
	operationScopeOpenProjectNote OperationID = "tui.scope.open-project-notes"

	operationProjectOpen    OperationID = "tui.project.open"
	operationProjectSetTags OperationID = "tui.project.set-tags"
	operationProjectDelete  OperationID = "tui.project.delete"
)

// projectOperationDescriptors is the authoritative catalog of project-targeted
// operations. Every entry resolves one frozen project identity, so the same
// descriptor serves the Scope Hub and any other host that names a
// project row.
func projectOperationDescriptors() []OperationDescriptor {
	descriptor := func(
		id OperationID,
		label string,
		description string,
		category OperationCategory,
		risk OperationRisk,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                id,
			Label:             label,
			Description:       description,
			Aliases:           aliases,
			Category:          category,
			TargetSource:      OperationTargetCurrentEntity,
			TargetCardinality: TargetCardinalityOne,
			Applicability:     projectOperationApplicable,
			Risk:              risk,
		}
	}
	return []OperationDescriptor{
		descriptor(operationProjectOpen, "Open project", "Focus the Outliner on this project", OperationCategoryProject, OperationRiskReadOnly, "focus project"),
		descriptor(operationScopeRenameProject, "Edit project", "Edit lean project metadata", OperationCategoryProject, OperationRiskMutation, "project settings"),
		descriptor(operationScopePinProject, "Toggle project pin", "Pin or unpin the project", OperationCategoryProject, OperationRiskMutation, "pin project", "unpin project"),
		descriptor(operationScopeArchiveProject, "Archive project", "Archive the project", OperationCategoryProject, OperationRiskDestructive),
		descriptor(operationScopeRestoreProject, "Unarchive project", "Return the project to active", OperationCategoryProject, OperationRiskMutation, "restore archived project"),
		descriptor(operationProjectDelete, "Delete project", "Delete the project and everything it owns", OperationCategoryProject, OperationRiskDestructive, "remove project", "destroy project"),
		descriptor(operationScopeOpenProjectNote, "Open project notes", "Open notes owned by this project", OperationCategoryNote, OperationRiskReadOnly),
		withValueProvider(
			descriptor(operationProjectSetTags, "Set project tags", "Edit the tags carried by this project", OperationCategoryProject, OperationRiskMutation, "project tags", "park project", "unpark project"),
			ValueProviderProjectTags,
		),
	}
}

// projectOperationApplicable admits a project operation only when the context
// names exactly one project. currentEntityIdentity degrades to the first scoped
// project when nothing is focused, so a multi-project scope is refused rather
// than allowed to borrow an unrelated project.
func projectOperationApplicable(ctx CommandContext) bool {
	// A section header names a lane, not a project. The scope fallback below would
	// hand it the sole scoped project, which is how a key meant for a project
	// header reaches one from a row that never named it.
	if ctx.Entity.HasSectionHeaderCursor {
		return false
	}
	if currentEntityIdentity(ctx).Kind != EntityKindProject {
		return false
	}
	if normalizeEntityIdentity(ctx.CurrentEntity).Kind == EntityKindProject || ctx.Entity.ProjectID != "" {
		return true
	}
	return len(normalizedProjectIDs(ctx.ScopeProjectIDs)) == 1
}

// projectTagsOperationResult round-trips the project's own tag set through the
// value picker: the first invocation asks for a set, the resumed one carries it.
// The chosen set replaces the stored set wholesale, so an empty slice is a
// legitimate value and only a missing one reopens the picker.
func projectTagsOperationResult(
	invocation OperationInvocation,
	project *models.Project,
) OperationHandlerResult {
	tags, ok := invocation.Value.([]string)
	if !invocation.HasValue || !ok {
		return OperationHandlerResult{Message: OperationValueRequestMsg{
			Request: OperationExecutionRequest{
				ID:      invocation.Descriptor.ID,
				Context: cloneCommandContext(invocation.Context),
			},
			Provider: ValueProviderProjectTags,
		}}
	}
	return OperationHandlerResult{Message: SetProjectTagsMsg{Project: project, Tags: tags}}
}

// projectOperationRequirement declares the project state one operation needs.
type projectOperationRequirement struct {
	requireArchived     bool
	requireActive       bool
	rejectSystemUnfiled bool
}

var projectOperationRequirements = map[OperationID]projectOperationRequirement{
	operationProjectOpen:          {requireActive: true},
	operationScopePinProject:      {requireActive: true},
	operationScopeArchiveProject:  {requireActive: true, rejectSystemUnfiled: true},
	operationScopeRestoreProject:  {requireArchived: true},
	operationScopeOpenProjectNote: {requireActive: true},
	operationProjectSetTags:       {requireActive: true},
	// Deleting carries no lifecycle requirement on purpose: an archived project
	// stays deletable, so archiving is a step toward removal rather than a state
	// that traps the project in the catalog.
	operationProjectDelete: {rejectSystemUnfiled: true},
}
