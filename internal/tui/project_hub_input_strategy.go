package tui

import "github.com/sglavoie/tt/internal/tui/operation"

// Project Hub surface operation IDs. They name what the Hub grammar does to its
// own list rather than to a project, so they stay apart from the tui.scope.* and
// tui.project.* operations a project row carries into an action sheet.
const (
	operationProjectHubSearch          operation.OperationID = "tui.project-hub.search"
	operationProjectHubToggleSelection operation.OperationID = "tui.project-hub.toggle-selection"
	operationProjectHubSelectAll       operation.OperationID = "tui.project-hub.select-all"
	operationProjectHubActions         operation.OperationID = "tui.project-hub.actions"
	operationProjectHubTaskFilter      operation.OperationID = "tui.project-hub.task-filter"
	operationProjectHubPreviewPageDown operation.OperationID = "tui.project-hub.preview-page-down"
	operationProjectHubPreviewPageUp   operation.OperationID = "tui.project-hub.preview-page-up"
)

// ProjectHubInputStrategy is the Project Hub's whole direct key grammar and the
// single source of its footer hints, `?` help, and dispatch. Administering a
// project is a direct key here rather than a menu row, so the six project
// operations the Hub reaches carry their own bindings and leave the Space menu
// to the two lifecycle actions and the bulk tag set.
//
// The cursor keys, Esc, `?`, and the workspace ledger's undo and redo resolve to
// the core operations because moving a list cursor, unwinding, asking what the
// keys do, and reversing the last transaction are the same acts everywhere; the
// Hub's own list vocabulary gets its own operations so the grammar names
// projects rather than tasks. Those core descriptors answer only to the
// Outliner, so they document the Hub's navigation without dispatching here: the
// Hub moves its own cursor, clears its own selection on Esc, raises its own help
// on `?`, hands u and Ctrl+R to the shell's one ledger, and the keymap beside
// this strategy carries their footer hints and help rows.
//
// u and Ctrl+R are here because a project mutation is a ledger transaction: the
// Hub archives, parks, renames, pins, and tags, and the status strip it already
// renders offers `u undo` after a bulk tag set. Binding them closes that
// promise rather than adding a new one.
func ProjectHubInputStrategy() operation.InputStrategy {
	return operation.InputStrategy{
		ID: operation.InputStrategyProjectHub,
		Bindings: []operation.OperationKeyBinding{
			{OperationID: operation.OperationProjectOpen, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 2, FooterLabel: "open"},
			{OperationID: operationProjectHubActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions"},
			{OperationID: operationScopeCreateProject, Keys: []string{"a"}, DisplayKey: "a", FooterRank: 6, FooterLabel: "new"},
			{OperationID: operationProjectHubToggleSelection, Keys: []string{"v"}, DisplayKey: "v", FooterRank: 7, FooterLabel: "select"},
			{OperationID: operation.OperationScopeRenameProject, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 9, FooterLabel: "edit"},
			{OperationID: operationProjectHubSearch, Keys: []string{"/"}, DisplayKey: "/", FooterRank: 12, FooterLabel: "search"},
			{OperationID: operation.OperationProjectSetTags, Keys: []string{"#"}, DisplayKey: "#", FooterRank: 14, FooterLabel: "project tags"},
			{OperationID: operation.OperationScopePinProject, Keys: []string{"*"}, DisplayKey: "*", FooterRank: 15, FooterLabel: "pin"},
			{OperationID: operation.OperationScopeOpenProjectNote, Keys: []string{"ctrl+n"}, DisplayKey: "Ctrl+N", FooterRank: 16, FooterLabel: "notes"},
			{OperationID: operationProjectHubSelectAll, Keys: []string{"ctrl+a"}, DisplayKey: "Ctrl+A", FooterRank: 17, FooterLabel: "select all"},
			{OperationID: operationProjectHubTaskFilter, Keys: []string{"ctrl+t"}, DisplayKey: "Ctrl+T"},
			// Ctrl+F/B page the preview pane rather than the list: Ctrl+D/U already own
			// the list cursor here, and the two panes scroll independently, so the two
			// halves of vim's paging vocabulary split between them.
			{OperationID: operationProjectHubPreviewPageDown, Keys: []string{"ctrl+f"}, DisplayKey: "Ctrl+F"},
			{OperationID: operationProjectHubPreviewPageUp, Keys: []string{"ctrl+b"}, DisplayKey: "Ctrl+B"},
			{OperationID: operation.OperationCoreUndo, Keys: []string{"u"}, DisplayKey: "u"},
			{OperationID: operation.OperationCoreRedo, Keys: []string{"ctrl+r"}, DisplayKey: "Ctrl+R"},
			{OperationID: operation.OperationCoreCursorDown, Keys: []string{"j", "down"}, DisplayKey: "j/↓"},
			{OperationID: operation.OperationCoreCursorUp, Keys: []string{"k", "up"}, DisplayKey: "k/↑"},
			{OperationID: operation.OperationCoreCursorTop, Keys: []string{"home"}, Sequence: []string{"g", "g"}, DisplayKey: "gg/Home", HintLabel: "top"},
			{OperationID: operation.OperationCoreCursorBottom, Keys: []string{"G", "end"}, DisplayKey: "G/End"},
			{OperationID: operation.OperationCoreCursorPageDown, Keys: []string{"ctrl+d"}, DisplayKey: "Ctrl+D"},
			{OperationID: operation.OperationCoreCursorPageUp, Keys: []string{"ctrl+u"}, DisplayKey: "Ctrl+U"},
			{OperationID: operation.OperationCoreHelp, Keys: []string{"?"}, DisplayKey: "?"},
			{OperationID: operation.OperationCoreBack, Keys: []string{"esc"}, DisplayKey: "Esc"},
		},
		ChordGroups: []operation.OperationChordGroup{
			{Sequence: []string{"g"}, HintLabel: "go to"},
		},
	}
}

// projectOperationManifest is the authoritative ordered Project Hub action list
// and the source the action sheet groups by. The thirteen operations the grammar
// reaches on press one are DirectOnly: they keep their descriptor, value
// provider and category, and leave the Space menu to archiving, unarchiving, and
// the bulk tag set.
//
// The seven list operations below the six project ones are here so the one-path
// rule sees them: a manifest is all ValidateOperationOnePath inspects, so an
// unmanifested operation is the one thing that gate cannot check. Listing them
// adds no menu row, because both sheets take an explicit operation list rather
// than this one; their category is stated so projectHubActionCategory keeps
// answering what it answered while they were absent.
func projectOperationManifest() []operation.OperationManifestEntry {
	return []operation.OperationManifestEntry{
		{ID: operation.OperationProjectOpen, Label: "Open project", Aliases: []string{"focus project"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationScopeCreateProject, Label: "New project", Aliases: []string{"create project"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationScopeRenameProject, Label: "Edit project", Aliases: []string{"project settings"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationScopePinProject, Label: "Toggle project pin", Aliases: []string{"pin project", "unpin project"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationProjectSetTags, Label: "Set project tags", Aliases: []string{"project tags"}, Provider: operation.ValueProviderProjectTags, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operation.OperationScopeOpenProjectNote, Label: "Open project notes", Category: projectHubActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationProjectHubSearch, Label: "Search projects", Aliases: []string{"filter the project list"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationProjectHubToggleSelection, Label: "Select project", Aliases: []string{"toggle project selection"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationProjectHubSelectAll, Label: "Select every matching project", Aliases: []string{"select all projects"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationProjectHubActions, Label: "Project actions", Aliases: []string{"project action sheet"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationProjectHubTaskFilter, Label: "Cycle every project, only those holding open tasks, only those holding none open", Aliases: []string{"projects holding open tasks", "projects holding no open tasks"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationProjectHubPreviewPageDown, Label: "Page project tasks down", Aliases: []string{"scroll project preview down"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationProjectHubPreviewPageUp, Label: "Page project tasks up", Aliases: []string{"scroll project preview up"}, Category: projectHubActionCategoryProject, Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationProjectBulkAddTag, Label: "Add tag", Category: projectHubActionCategoryProject, Risk: operation.OperationRiskMutation, Bulk: true},
		{ID: operationProjectBulkRemoveTag, Label: "Remove tag", Category: projectHubActionCategoryProject, Risk: operation.OperationRiskMutation, Bulk: true},
		{ID: operationProjectBulkPark, Label: "Park projects", Category: projectHubActionCategoryProject, Risk: operation.OperationRiskMutation, Bulk: true},
		{ID: operationProjectBulkUnpark, Label: "Unpark projects", Category: projectHubActionCategoryProject, Risk: operation.OperationRiskMutation, Bulk: true},
		{ID: operation.OperationScopeRestoreProject, Label: "Unarchive project", Aliases: []string{"restore archived project"}, Category: projectHubActionCategoryRecovery, Risk: operation.OperationRiskMutation},
		{ID: operation.OperationScopeArchiveProject, Label: "Archive project", Category: projectHubActionCategoryDanger, Risk: operation.OperationRiskDestructive},
	}
}

// validateProjectOperationOnePath holds the Project Hub to the shared one-path
// rule: a manifested operation is either bound directly and marked DirectOnly or
// listed in the Space menu, never both.
func validateProjectOperationOnePath(strategy operation.InputStrategy) error {
	return operation.ValidateOperationOnePath(projectOperationManifest(), strategy)
}
