package operation

// AvailabilityReason explains why an action cannot run, or returns empty when
// it can. Discovery and invocation both read the same applicability, identity,
// and archive policies; the executor remains the mutation authority.
func AvailabilityReason(descriptor OperationDescriptor, ctx CommandContext) string {
	if !descriptor.Applicable(ctx) {
		return unavailableReason(descriptor, ctx)
	}
	if err := archivedTaskTargetRejection(descriptor, ctx); err != nil {
		return err.Error()
	}
	if _, err := (OperationTargetResolver{}).Resolve(descriptor, ctx); err != nil {
		return unavailableReason(descriptor, ctx)
	}
	return ""
}

func unavailableReason(descriptor OperationDescriptor, ctx CommandContext) string {
	if descriptor.TargetSource == OperationTargetGlobal || descriptor.TargetSource == OperationTargetScopedProjects {
		return "This action is unavailable in the current view."
	}
	if ctx.Entity.Archived && descriptor.Risk != OperationRiskReadOnly {
		return "This project is archived. Open an active project to make changes."
	}
	if descriptor.ID == OperationCoreEdit && ctx.Selection.Visual {
		return "Leave multi-select, then choose one task to edit."
	}
	if descriptor.TargetSource == OperationTargetCurrentProject {
		return "Choose a project header to use this action."
	}
	if ctx.Entity.HasChecklistCursor {
		return "Choose the task row to use this action."
	}
	if ctx.Selection.Visual && ctx.Selection.Count() == 0 {
		return "Select at least one task."
	}
	if ctx.Entity.TaskID == "" && ctx.Selection.Count() == 0 {
		if descriptor.TargetSource == OperationTargetCurrentEntity {
			return "Choose a task or project header to use this action."
		}
		return "Choose a task to use this action."
	}
	if ctx.Selection.Visual {
		return "Leave multi-select to use this action on one task."
	}
	return "This action is unavailable for the current selection."
}
