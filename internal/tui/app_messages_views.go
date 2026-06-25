package tui

import "github.com/sglavoie/tt/internal/models"

// =============================================================================
// Settings Messages
// =============================================================================

// SettingsMsg opens the settings view.
type SettingsMsg struct{}

// SettingsSavedMsg signals settings were saved.
type SettingsSavedMsg struct{}

// SettingsStagesMsg opens the settings view on the Stages tab.
type SettingsStagesMsg struct{}

// =============================================================================
// Priority View Messages (Show/Close Pattern)
// =============================================================================

// ShowPriorityViewMsg opens the priority overview.
type ShowPriorityViewMsg struct {
	Project *models.Project
}

// ClosePriorityViewMsg closes the priority overview.
type ClosePriorityViewMsg struct{ PopViewMsg }

// ShowScoreEditorMsg opens the score dimension editor for a task.
type ShowScoreEditorMsg struct {
	Task    *models.Task
	Project *models.Project
}

// PriorityViewSelectMsg navigates to a task from priority view in the project tree.
type PriorityViewSelectMsg struct {
	Task *models.Task
}

// =============================================================================
// Dashboard Messages
// =============================================================================

// ToggleInfoPanelMsg toggles the info panel visibility in dashboard.
type ToggleInfoPanelMsg struct{}

// =============================================================================
// Daily Review Messages (Show/Close Pattern)
// =============================================================================

// ShowDailyReviewMsg opens the daily review view.
type ShowDailyReviewMsg struct{}

// CloseDailyReviewMsg closes the daily review view.
type CloseDailyReviewMsg struct{ PopViewMsg }

// DailyReviewRefreshMsg requests a refresh of the daily review data.
type DailyReviewRefreshMsg struct{}

// DailyReviewSelectMsg navigates to a task from daily review.
type DailyReviewSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// MarkDailyReviewDoneMsg signals that the user has completed their daily review.
type MarkDailyReviewDoneMsg struct{}

// MarkWeeklyReviewDoneMsg signals that the user has completed their weekly review.
type MarkWeeklyReviewDoneMsg struct{}

// ResetDailyReviewMsg signals that the user wants to reset their daily review completion.
type ResetDailyReviewMsg struct{}

// ResetWeeklyReviewMsg signals that the user wants to reset their weekly review completion.
type ResetWeeklyReviewMsg struct{}

// =============================================================================
// Focus View Messages (Show/Close Pattern)
// =============================================================================

// ShowFocusViewMsg opens the focus view.
type ShowFocusViewMsg struct{}

// FocusViewRefreshMsg requests a refresh of the focus view data.
type FocusViewRefreshMsg struct{}

// FocusViewSelectMsg navigates to a task from focus view.
type FocusViewSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// =============================================================================
// Recent Activity Messages (Show/Close Pattern)
// =============================================================================

// ShowRecentActivityMsg opens the recent activity view.
type ShowRecentActivityMsg struct{}

// CloseRecentActivityMsg closes the recent activity view.
type CloseRecentActivityMsg struct{ PopToAnchorMsg }

// ShowRecentActivityPreviewMsg opens a task preview from recent activity.
type ShowRecentActivityPreviewMsg struct {
	Task    CrossProjectTask
	Project *models.Project
}

// CloseRecentActivityPreviewMsg closes the recent activity task preview.
type CloseRecentActivityPreviewMsg struct{ PopViewMsg }

// RecentActivitySelectMsg navigates to a task from recent activity.
type RecentActivitySelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// ShowProjectRecentActivityMsg opens the project-scoped recent activity view.
type ShowProjectRecentActivityMsg struct {
	Project *models.Project
}

// CloseProjectRecentActivityMsg closes the project-scoped recent activity view.
type CloseProjectRecentActivityMsg struct{ PopViewMsg }

// =============================================================================
// Project Focus View Messages (Show/Close Pattern)
// =============================================================================

// ShowProjectFocusViewMsg opens the project-scoped focus view.
type ShowProjectFocusViewMsg struct {
	Project *models.Project
}

// CloseProjectFocusViewMsg closes the project focus view.
type CloseProjectFocusViewMsg struct{ PopViewMsg }

// =============================================================================
// Tutor Messages
// =============================================================================

// ShowTutorMsg opens the tutor view.
type ShowTutorMsg struct{}

// TutorDismissedMsg dismisses the tutor prompt on the dashboard.
type TutorDismissedMsg struct{}

// =============================================================================
// Today Messages (Show/Close Pattern)
// =============================================================================

// ShowFocusBoardMsg opens the Today view.
type ShowFocusBoardMsg struct{}

// CloseFocusBoardMsg closes the Today view.
type CloseFocusBoardMsg struct{ PopToAnchorMsg }

// FocusBoardRefreshMsg requests a refresh of the Today data.
type FocusBoardRefreshMsg struct{}

// FocusBoardSelectMsg navigates to a task from Today.
type FocusBoardSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// =============================================================================
// Score Profile Manager Messages (Show/Close Pattern)
// =============================================================================

// ShowScoreProfileManagerMsg opens the scoring profile manager view.
type ShowScoreProfileManagerMsg struct {
	ProjectID string
}

// CloseScoreProfileManagerMsg closes the scoring profile manager view.
type CloseScoreProfileManagerMsg struct{ PopViewMsg }

// ScoreProfileChangedMsg signals a scoring profile was added, deleted, assigned, or unassigned.
type ScoreProfileChangedMsg struct{}

// =============================================================================
// Outliner Messages (Show/Close Pattern)
// =============================================================================

// ShowOutlinerMsg opens the outliner view for a project.
// When Temporary is true, the handler pushes a fresh OutlinerModel via
// showView() without assigning it to AppModel.outliner, so the existing
// single-project outliner session is preserved. OpenAtTask, when non-nil,
// positions the cursor on that task once the row list is first built.
type ShowOutlinerMsg struct {
	Project    *models.Project
	OpenAtTask *models.Task
	Temporary  bool
}

// CloseOutlinerMsg closes the outliner view.
type CloseOutlinerMsg struct{}

// ShowGlobalOutlinerMsg opens the cross-project outliner view.
type ShowGlobalOutlinerMsg struct{}

// CloseGlobalOutlinerMsg closes the cross-project outliner and pops to the previous anchor.
type CloseGlobalOutlinerMsg struct{}

// CloseCrossOutlinerMsg closes the cross-project outliner and pops to the previous anchor
// without resetting the model state (cursor, perspective, fold state are preserved).
type CloseCrossOutlinerMsg struct{}

// =============================================================================
// Alert Center Messages (Show/Close Pattern)
// =============================================================================

// ShowNotificationCenterMsg opens the alert center view.
type ShowNotificationCenterMsg struct{}

// CloseNotificationCenterMsg closes the alert center view.
type CloseNotificationCenterMsg struct{ PopViewMsg }

// DismissAlertMsg requests dismissal of a specific alert.
type DismissAlertMsg struct {
	Key      string
	Category string
}

// UndismissAlertMsg requests removal of a dismissal for a specific alert.
type UndismissAlertMsg struct {
	Key string
}

// =============================================================================
// Kanban Messages (Show/Close Pattern)
// =============================================================================

// ShowKanbanMsg opens the per-project kanban view.
type ShowKanbanMsg struct {
	Project *models.Project
}

// ShowCrossProjectKanbanMsg opens the cross-project kanban view.
type ShowCrossProjectKanbanMsg struct{}

// KanbanSelectMsg navigates to a task's project from the kanban view.
type KanbanSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}
