package tui

import tea "github.com/charmbracelet/bubbletea"

// View is the base interface that all TUI views must implement.
// It extends tea.Model with dimension management.
type View interface {
	tea.Model
	SetSize(width, height int)
}

// SizableView extends View with dimension accessors.
// Use this when a view needs to report its dimensions to parent views.
type SizableView interface {
	View
	Width() int
	Height() int
}

// SearchableView is implemented by views that support text search.
type SearchableView interface {
	View
	HasActiveSearch() bool
	ClearSearch()
}

// ErrorView is implemented by views that can display errors.
type ErrorView interface {
	View
	SetError(err error)
	ClearError()
	HasError() bool
	Error() error
}

// FocusableView is implemented by views that manage input focus.
type FocusableView interface {
	View
	Focus()
	Blur()
	IsFocused() bool
}

// ViewContext provides shared state and dependencies to views.
type ViewContext struct {
	Width         int
	Height        int
	Config        interface{} // *config.Config
	ConfigManager interface{} // *config.Manager
}
