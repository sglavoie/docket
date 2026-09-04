package projecthub

import "github.com/sglavoie/tt/internal/tui/operation"

// Bulk project tag operation IDs. They are separate descriptors rather than one
// polymorphic tag editor because each carries its own label into the sheet, and
// because Park and Unpark take no value at all while Add and Remove take a tag
// name.
const (
	operationProjectBulkAddTag    operation.OperationID = "tui.project.bulk-add-tag"
	operationProjectBulkRemoveTag operation.OperationID = "tui.project.bulk-remove-tag"
	operationProjectBulkPark      operation.OperationID = "tui.project.bulk-park"
	operationProjectBulkUnpark    operation.OperationID = "tui.project.bulk-unpark"
)

// projectBulkOperations is the closed, ordered set of actions a project
// multi-selection may run. Restricting the bulk sheet to this list is the same
// rule the Outliner applies to bulk task actions: an operation reaches a
// multi-target sheet only when one transaction can carry every frozen target.
var projectBulkOperations = []operation.OperationID{
	operationProjectBulkAddTag,
	operationProjectBulkRemoveTag,
	operationProjectBulkPark,
	operationProjectBulkUnpark,
}

var ProjectBulkTagActions = map[operation.OperationID]ProjectBulkTagAction{
	operationProjectBulkAddTag:    ProjectBulkTagAdd,
	operationProjectBulkRemoveTag: ProjectBulkTagRemove,
	operationProjectBulkPark:      ProjectBulkTagPark,
	operationProjectBulkUnpark:    ProjectBulkTagUnpark,
}

func ProjectBulkOperationDescriptors() []operation.OperationDescriptor {
	descriptor := func(
		id operation.OperationID,
		label string,
		description string,
		aliases ...string,
	) operation.OperationDescriptor {
		return operation.OperationDescriptor{
			ID:                   id,
			Label:                label,
			Description:          description,
			Aliases:              aliases,
			Category:             operation.OperationCategoryProject,
			TargetSource:         operation.OperationTargetSelectedEntities,
			TargetCardinality:    operation.TargetCardinalityOneOrMore,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityProjectHub),
			Applicability:        ProjectBulkTagsApplicable,
			Risk:                 operation.OperationRiskMutation,
		}
	}
	return []operation.OperationDescriptor{
		operation.WithValueProvider(
			descriptor(operationProjectBulkAddTag, "Add tag", "Add one tag to every selected project", "bulk tag projects"),
			operation.ValueProviderTagName,
		),
		operation.WithValueProvider(
			descriptor(operationProjectBulkRemoveTag, "Remove tag", "Remove one tag from every selected project", "bulk untag projects"),
			operation.ValueProviderTagName,
		),
		descriptor(operationProjectBulkPark, "Park projects", "Hide every selected project from Active Projects"),
		descriptor(operationProjectBulkUnpark, "Unpark projects", "Return every selected project to Active Projects"),
	}
}

// ProjectBulkTagsApplicable admits a bulk tag action only when the context
// carries an explicit project multi-selection. A single-row sheet therefore
// never offers them, and no bulk action can borrow the cursor's project the way
// a degrading target source would.
func ProjectBulkTagsApplicable(ctx operation.CommandContext) bool {
	selected := operation.NormalizedEntityIdentities(ctx.Selection.EntityIdentities)
	if len(selected) == 0 {
		return false
	}
	for _, identity := range selected {
		if identity.Kind != operation.EntityKindProject {
			return false
		}
	}
	return true
}
