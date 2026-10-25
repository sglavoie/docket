package operation

import (
	"github.com/sglavoie/docket/internal/tui/presentation"
)

func CoreOutlinerApplicability(ctx CommandContext) bool {
	return ctx.View == presentation.ViewCrossProjectOutliner
}

// CoreInputOperationRequiresTask names the delegated core operations whose
// handler branch does nothing at all without a task. Delegated bindings bypass
// registry target resolution, so an unmet requirement reaches the user as a
// silent no-op unless it is refused here.
func CoreInputOperationRequiresTask(id OperationID) bool {
	switch id { //nolint:exhaustive // Only the task-consuming core operations are listed.
	case OperationCoreEdit,
		OperationCoreEditDescription,
		OperationCoreExternalEditor,
		OperationCoreBacklog,
		OperationCoreToday,
		OperationCoreNext,
		OperationCoreFocusSubtree,
		OperationCoreFocusParent,
		OperationCoreGotoParentTask,
		OperationCoreGotoTaskProject,
		OperationCoreGotoRelation:
		return true
	default:
		return false
	}
}

// CoreInputOperationRequiresPeekTarget names the delegated core operations that
// need a row with a detail document behind it. They are the task requirement's
// sibling rather than a member of it: Peek reads several entity kinds, so a project
// header answers these five and refuses every other task key on the same row.
func CoreInputOperationRequiresPeekTarget(id OperationID) bool {
	switch id { //nolint:exhaustive // Only Peek and its four scrolling keys are listed.
	case OperationCorePeek,
		OperationCorePeekHalfPageDown,
		OperationCorePeekHalfPageUp,
		OperationCorePeekPageDown,
		OperationCorePeekPageUp:
		return true
	default:
		return false
	}
}

// CommandContextNamesPeekTarget answers whether the cursor names something Peek
// can draw. It is the one predicate behind the descriptor's applicability and
// the delegated handler's refusal, so a key that is offered is a key that acts.
func CommandContextNamesPeekTarget(ctx CommandContext) bool {
	return CurrentTaskIdentity(ctx).Valid() || ctx.Entity.HasProjectHeaderCursor
}

// CommandContextSatisfiesPeekRequirement is what those five keys actually need,
// and it is the toggle that differs: closing an open dock names nothing, because
// the dock is the frame's rather than the row's. So Peek answers on any row
// while one is on screen — a reader who opened the dock and then moved onto a
// section header still has one key to close it with — while opening it, and
// every scrolling key, still needs a row whose detail Peek can draw. Both the
// descriptor's applicability and the delegated handler's refusal ask here, so
// the key that is offered stays the key that acts.
func CommandContextSatisfiesPeekRequirement(id OperationID, ctx CommandContext) bool {
	if id == OperationCorePeek && ctx.Peek.Visible {
		return true
	}
	return CommandContextNamesPeekTarget(ctx)
}

// openProjectApplicability is Enter's project branch: it answers on a project
// group header and on nothing else, which it establishes by ruling out every
// other row kind Enter can land on. The focused-project header is one of those:
// it names the scope the Outliner is already in, so Open Project has nowhere to
// open, and refusing here rather than no-opping in the handler is what keeps the
// footer from offering Enter on the one row where it does nothing.
func openProjectApplicability(ctx CommandContext) bool {
	return CoreOutlinerApplicability(ctx) &&
		!ctx.Selection.Visual &&
		!ctx.Entity.HasChecklistCursor &&
		!ctx.Entity.HasSectionHeaderCursor &&
		!ctx.Entity.HasFocusedProjectHeaderCursor &&
		!ctx.Entity.HasNoteCursor &&
		!CurrentTaskIdentity(ctx).Valid()
}
