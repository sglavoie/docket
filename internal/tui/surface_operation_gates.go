package tui

import "github.com/sglavoie/tt/internal/tui/operation"

// surfaceOperationGate names one entity surface and, once the surface manifests
// its operations, the startup check that holds it to the one-path rule.
type surfaceOperationGate struct {
	strategy operation.InputStrategy
	validate func(operation.InputStrategy) error
}

// surfaceOperationGates is the inventory of entity surfaces the one-operation
// grammar governs.
//
// It stays in the shell rather than beside ValidateOperationOnePath because
// every entry names a surface strategy and a surface validator, and those live
// above the operation package rather than inside it.
func surfaceOperationGates() []surfaceOperationGate {
	return []surfaceOperationGate{
		{strategy: SimplifiedInputStrategy(), validate: validateOutlinerOperationOnePath},
		{strategy: ProjectHubInputStrategy(), validate: validateProjectOperationOnePath},
		{strategy: NotesInputStrategy(), validate: validateNotesOperationOnePath},
		{strategy: AttachmentsInputStrategy(), validate: validateAttachmentOperationOnePath},
		{strategy: CommentsInputStrategy(), validate: validateCommentOperationOnePath},
	}
}
