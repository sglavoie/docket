package tui

// Layout constants for TUI rendering dimensions and spacing.
// These replace hardcoded magic numbers throughout the view code.

// ColumnLayout holds pre-computed fixed-width column measurements for a single
// render pass. When non-nil, badge columns are padded to fixed widths so
// metadata aligns vertically across all rows. Nil = current variable behavior.
type ColumnLayout struct {
	PriorityWidth int // 1 when any row shows priority badge; 0 if all hidden
	StatusWidth   int // max rendered status badge visual width; 0 when column inactive
	ProjectWidth  int // max rendered "[project]" badge visual width (cross-project views); 0 when not in use
	HasPriority   bool
	HasStatus     bool
	HasProject    bool
}

const (
	// TabbedViewChrome is the vertical chrome for views with a title, tab
	// bar (with border), blank line, footer, and padding.
	TabbedViewChrome = 6

	// OverlayListChrome is the vertical chrome for overlay/picker views
	// that have a title, search bar, borders, help line, and footer.
	OverlayListChrome = 8

	// MaxContentWidth is the maximum width for centered content areas
	// (e.g., task-focused tools).
	MaxContentWidth = 100

	// SeparatorWidth is the width of a vertical separator between panes.
	SeparatorWidth = 3

	// CreatorTabBarPadding is the horizontal padding subtracted from width
	// when rendering tab bars in creator/editor views.
	CreatorTabBarPadding = 4

	// MinColumnAlignWidth is the minimum render width at which column alignment
	// is applied. Below this threshold prefix padding is skipped to preserve space.
	MinColumnAlignWidth = 60
)
