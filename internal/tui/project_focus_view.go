// Package tui provides the terminal user interface for tt.
//
// Project Focus View
// ==================
// The project focus view displays queued tasks for a single project.
// It is split across multiple files:
//
//   - project_focus_types.go: Model struct, message types, and interface implementations
//   - project_focus_data.go: Constructor, task loading, filtering, sub-tab computation
//   - project_focus_update.go: Update() method and key handling
//   - project_focus_render.go: View() method and rendering helpers
//   - project_focus_navigation.go: Navigation-specific logic
package tui
