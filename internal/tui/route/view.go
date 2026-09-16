package route

import (
	"image/color"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
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

// ContextualGuidanceProvider is implemented by views that supply structured
// guidance for the app shell. The shell renders it above the pinned-task bar
// and generic breadcrumb but below transient loading/error/status overlays.
// Returning ok=false lets the shell fall through to lower-priority chrome.
type ContextualGuidanceProvider interface {
	ContextualGuidance() (spec chrome.GuidanceSpec, ok bool)
}

// SelfGuidedPresentation is the other half of the guidance-provider contract: a
// view says here that it has already drawn its instructions inside its own
// content, so the shell footer must not publish a second set underneath it.
//
// It exists because a PresentationID names a view, not a presentation, and one
// view may own two. The task composer is that case — a compact panel the shell
// speaks for, and an expanded form that draws its own footer — so the static
// policy cannot answer for it and the view answers per frame instead. A view
// that never implements this is answered by ChromePolicyForPresentation alone.
type SelfGuidedPresentation interface {
	PublishesOwnGuidance() bool
}

// ViewStatusStripProvider is implemented by views that own a status strip the
// shell pins directly above the footer, under its own dim divider. The view
// reserves the strip's height in its own layout budget; the shell only
// composes it. An empty return suppresses the strip and its divider.
type ViewStatusStripProvider interface {
	ViewStatusStrip(width int) string
}

// HostedPanelView is implemented by a view that presents as a bounded panel
// pinned to the bottom of the content area, drawn over the presentation it was
// opened from rather than replacing it. It is how one view declares which of its
// two presentations is live: a compact panel, or the whole content area.
//
// The shell is the only caller. It hands the host the rows the panel left it and
// re-renders it there, so the host stays a live projection under the panel
// instead of a covered one — the row its cursor is on is never hidden.
type HostedPanelView interface {
	HostedPanelLayout(width, height int) chrome.HostedPanelLayout
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
