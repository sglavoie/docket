// Package tui provides the terminal user interface for tt.
//
// This file serves as the entry point for the notification center module.
// The implementation is split across multiple files:
//   - notification_center_types.go: Model struct, item type, constructor
//   - notification_center_builder.go: Item building and sorting logic
//   - notification_center_navigation.go: Cursor movement and selection helpers
//   - notification_center_update.go: Update method and key handlers
//   - notification_center_render.go: View method and rendering helpers
package tui

import "github.com/sglavoie/tt/internal/alerts"

// AlertCenterNavigateMsg requests navigation from the alert center to a task/project.
type AlertCenterNavigateMsg struct {
	TaskID    string
	ProjectID string
	Category  alerts.AlertCategory
}

// StatusBarLabel returns the label for this view in the status bar.
func (m *NotificationCenterModel) StatusBarLabel() string { return "Notifications" }
