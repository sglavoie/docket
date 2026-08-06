package tui

import (
	"image/color"

	tea "charm.land/bubbletea/v2"
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
	// (no external status bar). Views like Help render their own footer and need
	// the entire terminal area.
	IsFullHeight() bool
}

// StatusBarFooterSpec describes a view's contextual footer as plain, unstyled
// strings. Left is the multi-item help cluster (items joined by " · " so the
// shared renderer can wrap on those boundaries); Right is a short
// indicator/label. Styling is applied once by renderContextualFooter, so
// providers must return unstyled text.
type StatusBarFooterSpec struct {
	Left  string
	Right string
}

// ContextualFooterProvider is implemented by views that supply their own
// contextual footer content for the app shell to compose. The app-shell status
// bar renders this spec at a priority above the pinned-task bar and the generic
// breadcrumb, but below transient loading/error/status overlays. Returning
// ok=false means the view has no contextual footer for its current state, so
// the app shell falls through to the lower-priority footers.
type ContextualFooterProvider interface {
	ContextualFooter() (spec StatusBarFooterSpec, ok bool)
}

// StatusBarLabeler is implemented by views that provide their own status bar label.
// Views without this interface show an empty label in the status bar.
type StatusBarLabeler interface {
	StatusBarLabel() string
}

// ViewExitResetter is implemented by views that need to reset interactive state
// (e.g. text inputs, edit modes) when popped from the view stack.
type ViewExitResetter interface {
	OnExit()
}

// ViewResumeHandler is implemented by views that need to restart visible-only
// work after a covering view is popped from the stack.
type ViewResumeHandler interface {
	OnResume() tea.Cmd
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
type ViewContext struct{}
