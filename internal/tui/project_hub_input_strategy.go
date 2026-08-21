package tui

// Project Hub surface operation IDs. They name what the Hub grammar does to its
// own list rather than to a project, so they stay apart from the tui.scope.* and
// tui.project.* operations a project row carries into an action sheet.
const (
	operationProjectHubSearch          OperationID = "tui.project-hub.search"
	operationProjectHubToggleSelection OperationID = "tui.project-hub.toggle-selection"
	operationProjectHubSelectAll       OperationID = "tui.project-hub.select-all"
	operationProjectHubActions         OperationID = "tui.project-hub.actions"
	operationProjectHubTaskFilter      OperationID = "tui.project-hub.task-filter"
)

// ProjectHubInputStrategy is the Project Hub's whole direct key grammar and the
// single source of its footer hints, `?` help, and dispatch. Administering a
// project is a direct key here rather than a menu row, so the six project
// operations the Hub reaches carry their own bindings and leave the Space menu
// to the two lifecycle actions and the bulk tag set.
//
// The cursor keys resolve to the core navigation operations because moving a
// list cursor is the same act everywhere; the Hub's own list vocabulary gets its
// own operations so the grammar names projects rather than tasks. Those core
// descriptors answer only to the Outliner, so they document the Hub's navigation
// without dispatching here: the Hub moves its own cursor and the keymap beside
// this strategy carries their footer hints.
func ProjectHubInputStrategy() InputStrategy {
	return InputStrategy{
		ID: InputStrategyProjectHub,
		Bindings: []OperationKeyBinding{
			{OperationID: operationProjectOpen, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 2, FooterLabel: "open"},
			{OperationID: operationProjectHubActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions"},
			{OperationID: operationScopeCreateProject, Keys: []string{"a"}, DisplayKey: "a", FooterRank: 6, FooterLabel: "new"},
			{OperationID: operationProjectHubToggleSelection, Keys: []string{"v"}, DisplayKey: "v", FooterRank: 7, FooterLabel: "select"},
			{OperationID: operationScopeRenameProject, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 9, FooterLabel: "edit"},
			{OperationID: operationProjectHubSearch, Keys: []string{"/"}, DisplayKey: "/", FooterRank: 12, FooterLabel: "search"},
			{OperationID: operationProjectSetTags, Keys: []string{"#"}, DisplayKey: "#", FooterRank: 14, FooterLabel: "project tags"},
			{OperationID: operationScopePinProject, Keys: []string{"*"}, DisplayKey: "*", FooterRank: 15, FooterLabel: "pin"},
			{OperationID: operationScopeOpenProjectNote, Keys: []string{"ctrl+n"}, DisplayKey: "Ctrl+N", FooterRank: 16, FooterLabel: "notes"},
			{OperationID: operationProjectHubSelectAll, Keys: []string{"ctrl+a"}, DisplayKey: "Ctrl+A", FooterRank: 17, FooterLabel: "select all"},
			{OperationID: operationProjectHubTaskFilter, Keys: []string{"ctrl+t"}, DisplayKey: "Ctrl+T"},
			{OperationID: operationCoreCursorDown, Keys: []string{"j", "down"}, DisplayKey: "j/↓"},
			{OperationID: operationCoreCursorUp, Keys: []string{"k", "up"}, DisplayKey: "k/↑"},
			{OperationID: operationCoreCursorTop, Keys: []string{"home"}, Sequence: []string{"g", "g"}, DisplayKey: "gg/Home", HintLabel: "top"},
			{OperationID: operationCoreCursorBottom, Keys: []string{"G", "end"}, DisplayKey: "G/End"},
			{OperationID: operationCoreCursorPageDown, Keys: []string{"ctrl+d"}, DisplayKey: "Ctrl+D"},
			{OperationID: operationCoreCursorPageUp, Keys: []string{"ctrl+u"}, DisplayKey: "Ctrl+U"},
		},
		ChordGroups: []OperationChordGroup{
			{Sequence: []string{"g"}, HintLabel: "go to"},
		},
	}
}

// projectOperationManifest is the authoritative ordered Project Hub action list
// and the source the action sheet groups by. The six operations the grammar
// reaches on press one are DirectOnly: they keep their descriptor, value
// provider and category, and leave the Space menu to archiving, unarchiving, and
// the bulk tag set.
func projectOperationManifest() []operationManifestEntry {
	return []operationManifestEntry{
		{ID: operationProjectOpen, Label: "Open project", Aliases: []string{"focus project"}, Category: projectHubActionCategoryProject, Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationScopeCreateProject, Label: "New project", Aliases: []string{"create project"}, Category: projectHubActionCategoryProject, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationScopeRenameProject, Label: "Edit project", Aliases: []string{"project settings"}, Category: projectHubActionCategoryProject, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationScopePinProject, Label: "Toggle project pin", Aliases: []string{"pin project", "unpin project"}, Category: projectHubActionCategoryProject, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationProjectSetTags, Label: "Set project tags", Aliases: []string{"project tags"}, Provider: ValueProviderProjectTags, Category: projectHubActionCategoryProject, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationScopeOpenProjectNote, Label: "Open project notes", Category: projectHubActionCategoryProject, Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationProjectBulkAddTag, Label: "Add tag", Category: projectHubActionCategoryProject, Risk: OperationRiskMutation, Bulk: true},
		{ID: operationProjectBulkRemoveTag, Label: "Remove tag", Category: projectHubActionCategoryProject, Risk: OperationRiskMutation, Bulk: true},
		{ID: operationProjectBulkPark, Label: "Park projects", Category: projectHubActionCategoryProject, Risk: OperationRiskMutation, Bulk: true},
		{ID: operationProjectBulkUnpark, Label: "Unpark projects", Category: projectHubActionCategoryProject, Risk: OperationRiskMutation, Bulk: true},
		{ID: operationScopeRestoreProject, Label: "Unarchive project", Aliases: []string{"restore archived project"}, Category: projectHubActionCategoryRecovery, Risk: OperationRiskMutation},
		{ID: operationScopeArchiveProject, Label: "Archive project", Category: projectHubActionCategoryDanger, Risk: OperationRiskDestructive},
	}
}

// validateProjectOperationOnePath holds the Project Hub to the shared one-path
// rule: a manifested operation is either bound directly and marked DirectOnly or
// listed in the Space menu, never both.
func validateProjectOperationOnePath(strategy InputStrategy) error {
	return validateOperationOnePath(projectOperationManifest(), strategy)
}
