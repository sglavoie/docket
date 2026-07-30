package tui

import "github.com/sglavoie/tt/internal/models"

// =============================================================================
// Outliner Messages (Show/Close Pattern)
// =============================================================================

// launchOutlinerMsg is emitted exactly once by AppModel.Init. Keeping launch
// reset distinct from ordinary ShowGlobalOutlinerMsg navigation prevents a
// later route back to the Outliner from accidentally clearing in-session mode,
// filters, search, scope, cursor, or selection.
type launchOutlinerMsg struct{}

// ShowOutlinerMsg opens the outliner view for a project. OpenAtTask, when
// non-nil, positions the cursor on that task once the row list is first built.
type ShowOutlinerMsg struct {
	Project    *models.Project
	OpenAtTask *models.Task
	Options    OutlinerOpenOptions
}

// CloseOutlinerMsg closes the outliner view.
type CloseOutlinerMsg struct{}

// ShowGlobalOutlinerMsg opens the cross-project outliner view.
type ShowGlobalOutlinerMsg struct {
	Options OutlinerOpenOptions
}

// CloseGlobalOutlinerMsg closes the cross-project outliner and pops to the previous anchor.
type CloseGlobalOutlinerMsg struct{}

// CloseCrossOutlinerMsg closes the cross-project outliner and pops to the previous anchor
// without resetting the model state (cursor, mode, and fold state are preserved).
type CloseCrossOutlinerMsg struct{}
