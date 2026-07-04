package tui

// Layout constants for TUI rendering dimensions and spacing.
// These replace hardcoded magic numbers throughout the view code.

// ColumnLayout holds pre-computed fixed-width column measurements for a single
// render pass. When non-nil, badge columns are padded to fixed widths so
// metadata aligns vertically across all rows. Nil = current variable behavior.
type ColumnLayout struct {
	PriorityWidth int // 1 when any row shows priority badge; 0 if all hidden
	ShortIDWidth  int // max rendered "#N" visual width; 0 if column not visible
	StatusWidth   int // max rendered status badge visual width; 0 when column inactive
	TypeWidth     int // max rendered type badge visual width; 0 when column inactive
	ProjectWidth  int // max rendered "[project]" badge visual width (cross-project views); 0 when not in use
	HasPriority   bool
	HasShortID    bool
	HasStatus     bool
	HasType       bool
	HasProject    bool
}

const (
	// CardBorderOverhead is the horizontal space consumed by card borders
	// and padding (left border + left padding + right padding + right border).
	CardBorderOverhead = 4

	// CardVerticalBorderOverhead is the vertical space consumed by card
	// top and bottom borders.
	CardVerticalBorderOverhead = 2

	// MinCardWidth is the minimum width for a content card to remain legible.
	MinCardWidth = 20

	// MinTitleTruncWidth is the minimum width for truncated task titles.
	MinTitleTruncWidth = 15

	// DashboardInfoSectionWideMinWidth is the minimum terminal width to
	// render info section cards side-by-side instead of stacked.
	DashboardInfoSectionWideMinWidth = 100

	// DashboardNarrowModeWidth is the threshold below which the dashboard
	// switches to narrow mode rendering.
	DashboardNarrowModeWidth = 50

	// DashboardMinVisibleProjects is the minimum number of project rows
	// to display in the dashboard.
	DashboardMinVisibleProjects = 3

	// TreeHeaderOffset is the number of lines consumed by the tree view
	// header chrome (title, view mode tabs, status stats, etc.).
	// Filter/sort info is now inline on the title line, so no extra line needed.
	TreeHeaderOffset = 7

	// TreeMinWidth is the minimum width for the tree pane in split layout.
	TreeMinWidth = 35

	// SidebarMinWidth is the minimum width for the sidebar pane.
	SidebarMinWidth = 25

	// SidebarContentOverhead is the horizontal overhead for sidebar content.
	SidebarContentOverhead = 3

	// DashboardBottomPaneHeight is the minimum-clamp height for the bottom
	// info section (summary/alerts cards) in the dashboard horizontal
	// layout. Session 2 will compute the actual height from the configured
	// horizontal split percentage and clamp the result against this value.
	DashboardBottomPaneHeight = 12

	// CompactCardHeight is the height reserved for an inert one-line
	// summary row in compact dashboard card mode.
	CompactCardHeight = 1

	// FocusTitleMaxWidth is the maximum width for truncated titles in focus mode indicators.
	FocusTitleMaxWidth = 40

	// TitleFooterOverhead is the vertical space consumed by the task detail
	// title line plus its two-line status-bar footer.
	TitleFooterOverhead = 3

	// TabbedViewChrome is the vertical chrome for views with a title, tab
	// bar (with border), blank line, footer, and padding.
	TabbedViewChrome = 6

	// OverlayListChrome is the vertical chrome for overlay/picker views
	// that have a title, search bar, borders, help line, and footer.
	OverlayListChrome = 8

	// MaxContentWidth is the maximum width for centered content areas
	// (e.g., task detail view).
	MaxContentWidth = 100

	// SeparatorWidth is the width of a vertical separator between panes.
	SeparatorWidth = 3

	// FocusBoardTitlePadding is the horizontal space reserved for non-title
	// columns (status, priority, tags, etc.) in focus board rows.
	FocusBoardTitlePadding = 40

	// FocusBoardMinTitleWidth is the minimum title width in focus board rows.
	FocusBoardMinTitleWidth = 20

	// FocusViewQueueTitlePadding is the horizontal space reserved for
	// non-title columns in the Focus View queue and suggested tabs.
	FocusViewQueueTitlePadding = 50

	// FocusViewStandardTitlePadding is the horizontal space reserved for
	// non-title columns in the Focus View standard tabs (By Project, By Due Date).
	FocusViewStandardTitlePadding = 45

	// FocusViewMinTitleWidth is the minimum title width in focus view rows.
	FocusViewMinTitleWidth = 10

	// CreatorTabBarPadding is the horizontal padding subtracted from width
	// when rendering tab bars in creator/editor views.
	CreatorTabBarPadding = 4

	// MinColumnAlignWidth is the minimum render width at which column alignment
	// is applied. Below this threshold prefix padding is skipped to preserve space.
	MinColumnAlignWidth = 60
)
