package tui

// Layout constants for TUI rendering dimensions and spacing.
// These replace hardcoded magic numbers throughout the view code.

const (
	// CardBorderOverhead is the horizontal space consumed by card borders
	// and padding (left border + left padding + right padding + right border).
	CardBorderOverhead = 4

	// CardVerticalBorderOverhead is the vertical space consumed by card
	// top and bottom borders.
	CardVerticalBorderOverhead = 2

	// DividerHeight is the height of a horizontal divider between sections.
	DividerHeight = 1

	// TabBarHeight is the height of a tab bar (text line + bottom border).
	TabBarHeight = 2

	// FilterBarHeight is the height of the filter bar when enabled.
	FilterBarHeight = 2

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

	// AlertsCardHeight is the fixed height for the alerts card in dashboard.
	AlertsCardHeight = 8

	// FocusTitleMaxWidth is the maximum width for truncated titles in focus mode indicators.
	FocusTitleMaxWidth = 40

	// TitleFooterOverhead is the vertical space consumed by a title bar and
	// a footer bar in fullscreen detail views.
	TitleFooterOverhead = 2

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
)
