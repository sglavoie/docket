package tui

import "github.com/sglavoie/tt/internal/models"

// =============================================================================
// Settings Messages
// =============================================================================

// SettingsMsg opens the settings view.
type SettingsMsg struct{}

// SettingsSavedMsg signals settings were saved.
type SettingsSavedMsg struct{}

// ShowScoreEditorMsg opens the score dimension editor for a task.
type ShowScoreEditorMsg struct {
	Task    *models.Task
	Project *models.Project
}

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
// Recent Activity Messages (Show/Close Pattern)
// =============================================================================

// ShowRecentActivityMsg opens the recent activity view.
type ShowRecentActivityMsg struct{}

// CloseRecentActivityMsg closes the recent activity view.
type CloseRecentActivityMsg struct{ PopToAnchorMsg }

// ShowRecentActivityPreviewMsg opens a task preview from recent activity.
type ShowRecentActivityPreviewMsg struct {
	Task         CrossProjectTask
	ProjectScope *models.Project
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
// Tutor Messages
// =============================================================================

// ShowTutorMsg opens the tutor view.
type ShowTutorMsg struct{}

// TutorDismissedMsg dismisses the tutor prompt on the dashboard.
type TutorDismissedMsg struct{}

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
