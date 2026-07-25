package tui

// Message Type Patterns
//
// This file documents the message type conventions used throughout the TUI.
// Messages are the primary mechanism for communication between views in BubbleTea.
//
// # Show/Close Pattern (Modal Lifecycle)
//
// Used for overlay views (modals, pickers, dialogs). The Show message opens
// the view with initial data, the Close message dismisses it.
//
//   - ShowXxxMsg: Opens the view, often carries Project/Task context
//   - CloseXxxMsg: Dismisses the view, may carry result data
//
// Examples:
//   - ShowCommandPaletteMsg / CloseCommandPaletteMsg
//   - ShowFilterMsg / CloseFilterMsg
//   - ShowHelpMsg / CloseHelpMsg
//   - ShowTagSelectorMsg (closes via TagSelectorDoneMsg)
//
// # Done Pattern (Picker Results)
//
// Used by picker/selector views that return a selection result. The Done
// message includes a Canceled field to distinguish user cancellation from
// successful selection.
//
//   - XxxDoneMsg: Carries selected value(s) and Canceled bool
//
// Examples:
//   - TagSelectorDoneMsg{Tags: []string, Canceled: bool}
//   - SectionPickerDoneMsg{Section: string, Canceled: bool}
//   - PriorityPickerDoneMsg{Priority: int, Canceled: bool}
//   - ContextScopeSelectedMsg{SelectedContext: string, Canceled: bool}
//
// # CRUD Pattern (Entity Lifecycle)
//
// Used to notify about entity state changes. Handlers typically refresh
// views and save changes to storage.
//
//   - XxxCreatedMsg: New entity was created
//   - XxxUpdatedMsg: Existing entity was modified
//   - XxxDeletedMsg: Entity was removed
//
// Examples:
//   - ProjectCreatedMsg, ProjectUpdatedMsg, ProjectDeletedMsg
//   - TaskCreatedMsg, TaskUpdatedMsg, TaskDeletedMsg
//   - CommentAddedMsg, CommentEditedMsg, CommentDeletedMsg
//
// # Select Pattern (Navigation)
//
// Used when a view selection should trigger navigation to another view
// or highlight a specific item.
//
//   - XxxSelectMsg: Navigate to or highlight the specified entity
//
// Examples:
//   - SelectProjectMsg: Switch to project in dashboard
//   - GlobalSearchSelectMsg: Navigate to search result
//   - TaskJumperSelectMsg: Jump to task in tree
//
// # Action Pattern (Command Execution)
//
// Used to trigger specific operations, often from command palette or
// keyboard shortcuts.
//
//   - XxxActionMsg: Execute action with parameters
//   - XxxMsg (verb): Trigger an operation
//
// Examples:
//   - CommandPaletteTaskActionMsg{Action: "complete", Task: ...}
//   - RefreshMsg, BackMsg, QuitMsg
//
// # File Organization
//
// Messages are defined close to their primary consumers:
//
//   - app_messages_*.go: Core navigation and shared app-level messages
//   - inbox_messages.go: Inbox processor specific
//   - bulk_edit_types.go: Bulk edit panel specific
//   - command_palette.go: Palette-specific messages (end of file)
//   - [view]_xxx.go: View-specific messages near the view
//
// When adding new messages:
//   1. Choose the appropriate pattern from above
//   2. Place near the view that generates/consumes it
//   3. Include doc comments explaining purpose
//   4. For Show/Close pairs, keep them together
//

// GlowViewDoneMsg signals that glow pager has exited and views should refresh.
// This is needed because tea.ExecProcess suspends the TUI, and views may need
// to re-render their layout after returning from an external process.
type GlowViewDoneMsg struct{}

// ViewDidAppearMsg is dispatched by popView() when the view being revealed is
// ViewCrossProjectOutliner. The outliner handles this by triggering an eager
// reload from disk so that tasks modified during a drill-down are reflected
// immediately on return.
type ViewDidAppearMsg struct{}
