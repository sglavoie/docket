package notes

import (
	"github.com/sglavoie/tt/internal/tui/operation"
)

// NoteOperationDescriptors is the authoritative catalog of note-targeted
// operations. Each one resolves the frozen note identity at execution, so the
// same descriptor serves the Notes manager and any other host that
// names a note row. It sits beside the grammar that binds these IDs rather than
// beside the shell registrar, because a descriptor is data and the registrar's
// half is the handler.
func NoteOperationDescriptors() []operation.OperationDescriptor {
	descriptor := func(
		id operation.OperationID,
		label string,
		description string,
		risk operation.OperationRisk,
		applicability operation.OperationApplicability,
	) operation.OperationDescriptor {
		return operation.OperationDescriptor{
			ID:                id,
			Label:             label,
			Description:       description,
			Category:          operation.OperationCategoryNote,
			TargetSource:      operation.OperationTargetCurrentEntity,
			TargetCardinality: operation.TargetCardinalityOne,
			Applicability:     applicability,
			Risk:              risk,
		}
	}
	note := func(
		id operation.OperationID,
		label string,
		description string,
		risk operation.OperationRisk,
	) operation.OperationDescriptor {
		return descriptor(id, label, description, risk, noteOperationApplicable)
	}
	return []operation.OperationDescriptor{
		note(OperationNoteOpen, "Open Note", "Open this note in the external editor", operation.OperationRiskMutation),
		operation.WithValueProvider(
			note(OperationNoteRename, "Rename Note", "Give this note a new title", operation.OperationRiskMutation),
			operation.ValueProviderNoteTitle,
		),
		note(OperationNoteMove, "Move Note", "Move this note to another location", operation.OperationRiskMutation),
		note(OperationNoteCopyTitle, "Copy Note Title", "Copy this note's title to the clipboard", operation.OperationRiskReadOnly),
		note(OperationNoteCopyPath, "Copy Note Path", "Copy this note's file path to the clipboard", operation.OperationRiskReadOnly),
		note(OperationNoteDelete, "Delete Note", "Delete this note", operation.OperationRiskDestructive),
	}
}

// noteOperationApplicable admits a note operation only when the focused entity
// is a note. CurrentEntityIdentity degrades to the focused task and then the
// scoped project, so an unguarded descriptor would borrow an unrelated target.
func noteOperationApplicable(ctx operation.CommandContext) bool {
	return operation.CurrentEntityIdentity(ctx).Kind == operation.EntityKindNote
}

// NoteOperationUnavailableWithParseError reports the restored actions that need
// parsed note content. Open and delete stay available so a malformed note can
// still be repaired or removed.
func NoteOperationUnavailableWithParseError(id operation.OperationID) bool {
	return id == OperationNoteMove
}
