package operation

// Project operation IDs. The tui.scope.* IDs predate this catalog and keep
// their identity because they already name the same action from the Scope Hub;
// tui.project.* is reserved for the maintenance actions added alongside them.
// They are `OperationID` vocabulary rather than descriptors, so they sit beside
// the registry that owns the type while the Hub catalog and the Outliner sheet
// that register them stay above it.
const (
	// OperationScopeCreateProject is the Hub's `a`. It predates the dedicated
	// Project Hub and keeps its identity for the same reason its siblings do.
	OperationScopeCreateProject OperationID = "tui.scope.create-project"

	OperationScopeRenameProject   OperationID = "tui.scope.rename-project"
	OperationScopePinProject      OperationID = "tui.scope.pin-project"
	OperationScopeArchiveProject  OperationID = "tui.scope.archive-project"
	OperationScopeRestoreProject  OperationID = "tui.scope.restore-project"
	OperationScopeOpenProjectNote OperationID = "tui.scope.open-project-notes"

	OperationProjectOpen    OperationID = "tui.project.open"
	OperationProjectSetTags OperationID = "tui.project.set-tags"
	OperationProjectDelete  OperationID = "tui.project.delete"

	// OperationProjectActions is the Outliner's Space on a project group header.
	// It lives in the tui.project.* namespace because it names a project, and
	// apart from the Hub's own sheet operation because the two surfaces offer
	// different rows: the Hub reaches its project keys directly and menus only
	// the lifecycle pair, while there the whole administration set is the menu.
	OperationProjectActions OperationID = "tui.project.actions"
)
