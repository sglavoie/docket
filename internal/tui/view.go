package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
)

// View is the base interface that all TUI views must implement.
// It extends tea.Model with dimension management.
type View interface {
	tea.Model
	SetSize(width, height int)
}

// ViewCapabilities describes view-level behaviors used by the app shell.
// Views can implement this to control global navigation and input handling.
type ViewCapabilities interface {
	// ShouldCaptureEsc returns true if the view wants to handle esc/q directly.
	ShouldCaptureEsc() bool
	// HasTextInput returns true if the view currently has active text input.
	HasTextInput() bool
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
	Config        *config.Config
	ConfigManager *config.Manager
	Persister     Persister
}
