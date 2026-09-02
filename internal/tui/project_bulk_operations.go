package tui

// Bulk project tag operation IDs. They are separate descriptors rather than one
// polymorphic tag editor because each carries its own label into the sheet, and
// because Park and Unpark take no value at all while Add and Remove take a tag
// name.
const (
	operationProjectBulkAddTag    OperationID = "tui.project.bulk-add-tag"
	operationProjectBulkRemoveTag OperationID = "tui.project.bulk-remove-tag"
	operationProjectBulkPark      OperationID = "tui.project.bulk-park"
	operationProjectBulkUnpark    OperationID = "tui.project.bulk-unpark"
)

// projectBulkOperations is the closed, ordered set of actions a project
// multi-selection may run. Restricting the bulk sheet to this list is the same
// rule the Outliner applies to bulk task actions: an operation reaches a
// multi-target sheet only when one transaction can carry every frozen target.
var projectBulkOperations = []OperationID{
	operationProjectBulkAddTag,
	operationProjectBulkRemoveTag,
	operationProjectBulkPark,
	operationProjectBulkUnpark,
}

var projectBulkTagActions = map[OperationID]ProjectBulkTagAction{
	operationProjectBulkAddTag:    ProjectBulkTagAdd,
	operationProjectBulkRemoveTag: ProjectBulkTagRemove,
	operationProjectBulkPark:      ProjectBulkTagPark,
	operationProjectBulkUnpark:    ProjectBulkTagUnpark,
}

func projectBulkOperationDescriptors() []OperationDescriptor {
	descriptor := func(
		id OperationID,
		label string,
		description string,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                   id,
			Label:                label,
			Description:          description,
			Aliases:              aliases,
			Category:             OperationCategoryProject,
			TargetSource:         OperationTargetSelectedEntities,
			TargetCardinality:    TargetCardinalityOneOrMore,
			RequiredCapabilities: NewCapabilitySet(CapabilityProjectHub),
			Applicability:        projectBulkTagsApplicable,
			Risk:                 OperationRiskMutation,
		}
	}
	return []OperationDescriptor{
		withValueProvider(
			descriptor(operationProjectBulkAddTag, "Add tag", "Add one tag to every selected project", "bulk tag projects"),
			ValueProviderTagName,
		),
		withValueProvider(
			descriptor(operationProjectBulkRemoveTag, "Remove tag", "Remove one tag from every selected project", "bulk untag projects"),
			ValueProviderTagName,
		),
		descriptor(operationProjectBulkPark, "Park projects", "Hide every selected project from Active Projects"),
		descriptor(operationProjectBulkUnpark, "Unpark projects", "Return every selected project to Active Projects"),
	}
}

// projectBulkTagsApplicable admits a bulk tag action only when the context
// carries an explicit project multi-selection. A single-row sheet therefore
// never offers them, and no bulk action can borrow the cursor's project the way
// a degrading target source would.
func projectBulkTagsApplicable(ctx CommandContext) bool {
	selected := normalizedEntityIdentities(ctx.Selection.EntityIdentities)
	if len(selected) == 0 {
		return false
	}
	for _, identity := range selected {
		if identity.Kind != EntityKindProject {
			return false
		}
	}
	return true
}
