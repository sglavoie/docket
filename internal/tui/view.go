package tui

import (
	"image/color"

	tea "charm.land/bubbletea/v2"

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
	// IsFullHeight returns true if the view should use the full terminal height
	// (no external status bar). Views like Help, Tutor, Ask, and Command Palette
	// render their own footer and need the entire terminal area.
	IsFullHeight() bool
}

// PendingPrefixView is implemented by views that participate in multi-key
// prefix sequences (e.g. filter prefix F, goto prefix g). When a prefix is
// pending, the app shell must skip global single-character shortcuts so the
// next key reaches the view's prefix dispatcher. Satisfied automatically by
// views embedding InteractiveViewState.
type PendingPrefixView interface {
	HasPendingPrefix() bool
}

// StatusBarLabeler is implemented by views that provide their own status bar label.
// Views without this interface show an empty label in the status bar.
type StatusBarLabeler interface {
	StatusBarLabel() string
}

// HelpShortcutSupporter is implemented by views that suppress the help shortcut
// hint from the status bar. Views without this interface show help text by default.
type HelpShortcutSupporter interface {
	SupportsHelpShortcut() bool
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

// ViewExitResetter is implemented by views that need to reset interactive state
// (e.g. text inputs, edit modes) when popped from the view stack.
type ViewExitResetter interface {
	OnExit()
}

// ZenModeView is implemented by views that support a zen focus mode. When
// IsZenMode returns true, the app shell suppresses transient status bar
// overlays so the view is presented without any chrome.
type ZenModeView interface {
	IsZenMode() bool
}

// ActiveFilterProvider is implemented by views that support filtering and can
// report how many filters are currently active. Used by the status bar to show
// a filter pill when filters are active.
type ActiveFilterProvider interface {
	ActiveFilterCount() int
}

// ModeIndicator is implemented by views that can be in a named mode (search,
// multi-select, zen, tunnel, etc.). The status bar shows a colored badge when
// a mode is active. Returns ("", nil) when in normal mode.
type ModeIndicator interface {
	ActiveMode() (label string, c color.Color)
}

// AccentColorSetter is implemented by views that support contextual accent color
// injection from the app shell. BaseView provides the default implementation.
type AccentColorSetter interface {
	SetAccentColor(c color.Color)
	AccentColor() color.Color
}

// ViewContext provides shared state and dependencies to views.
type ViewContext struct {
	Width         int
	Height        int
	Config        *config.Config
	ConfigManager *config.Manager
	Persister     Persister
}
