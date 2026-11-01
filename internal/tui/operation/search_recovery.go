package operation

// Recovery changes the search's population, never its task targets. The same
// availability gates serve the picker, menu, help, and executor.
func searchRecoveryDescriptor(descriptor OperationDescriptor) OperationDescriptor {
	descriptor.Applicability = func(ctx CommandContext) bool {
		if !CoreOutlinerApplicability(ctx) || !ctx.Search.Ranked || ctx.Search.Query == "" {
			return false
		}
		switch descriptor.ID {
		case OperationCoreSearchFinished:
			return !ctx.Search.Pending && ctx.Search.Completed > 0
		case OperationCoreSearchDeferred:
			return !ctx.Search.Pending && ctx.Search.Deferred > 0
		default:
			return true
		}
	}
	return descriptor
}

func searchRecoveryUnavailableReason(id OperationID, ctx CommandContext) string {
	if ctx.Search.Query == "" {
		return "Enter a ranked task search with / first."
	}
	if !ctx.Search.Ranked {
		return "Press or to return to ranked search results first."
	}
	if ctx.Search.Pending {
		return "Search is still running. Wait for results, then try again."
	}
	if id == OperationCoreSearchFinished {
		return "No matching finished tasks are withheld in this scope and filters."
	}
	return "No matching deferred tasks are withheld in this scope and filters."
}
