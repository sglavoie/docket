package tui

import "github.com/charmbracelet/lipgloss"

// makeBadgeStyle creates a badge style with the given foreground and background ANSI colors.
func makeBadgeStyle(fg, bg string) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg))
}

// makeViewModeStyle creates a view mode tab style with the given background ANSI color.
func makeViewModeStyle(bg string) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(bg)).
		Foreground(lipgloss.Color("15")).
		Bold(true).
		Padding(0, 1)
}

// makeColorStyle creates a style with just a foreground color.
func makeColorStyle(color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(color)
}

// Color palette
var (
	ColorPrimary     = lipgloss.Color("39")      // Blue
	ColorSecondary   = lipgloss.Color("33")      // Cyan
	ColorSuccess     = lipgloss.Color("42")      // Green
	ColorWarning     = lipgloss.Color("214")     // Orange
	ColorError       = lipgloss.Color("196")     // Red
	ColorMuted       = lipgloss.Color("241")     // Gray
	ColorHighlight   = lipgloss.Color("229")     // Light yellow
	ColorBackground  = lipgloss.Color("236")     // Dark gray
	ColorBorder      = lipgloss.Color("240")     // Border gray
	ColorDiffContext = lipgloss.Color("245")     // Diff context gray
	ColorPrefixPopup = lipgloss.Color("226")     // Yellow for prefix popup border
	ColorSelectedBg  = lipgloss.Color("#4c0163") // Light purple background for selections
	ColorSelectedFg  = lipgloss.Color("#fbeeff") // Dark purple foreground for selections
	ColorFlashBg     = lipgloss.Color("#7b2fa0") // Brighter purple for cursor flash highlight
)

// Base styles
var (
	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			MarginBottom(1)

	// Text styles
	NormalStyle = lipgloss.NewStyle()

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	BoldStyle = lipgloss.NewStyle().
			Bold(true)

	// Status colors
	TodoStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	InProgressStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	ReviewStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	DoneStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	BlockedStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	StandbyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141"))

	CanceledStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Strikethrough(true)

	OverdueStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	DueSoonStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	DependencyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	// Priority styles
	PriorityHighStyle = lipgloss.NewStyle().
				Foreground(ColorError) // Red for priority 1

	PriorityMediumStyle = lipgloss.NewStyle().
				Foreground(ColorWarning) // Orange for priority 2

	PriorityLowStyle = lipgloss.NewStyle().
				Foreground(ColorMuted) // Dimmed gray for priority 3-5

	// Due date styles
	DueDateStyle = lipgloss.NewStyle().
			Foreground(ColorMuted) // Gray for normal due dates

	DueTodayStyle = lipgloss.NewStyle().
			Foreground(ColorWarning) // Orange for tasks due today

	// BlockedByDepsStyle is used for tasks with incomplete dependencies
	// Uses red background to create a visual "outline" effect on the status icon
	BlockedByDepsStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("231")). // White text
				Background(ColorError)             // Red background

	// Task type colors
	EpicStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("135")) // Purple

	StoryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")) // Blue

	TaskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")) // Green

	BugStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	IdeaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")) // Orange

	NoteStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("248")) // Gray

	// Task type badge styles (icon with background for visual distinction)
	EpicBadgeStyle  = makeBadgeStyle("135", "53")  // Purple
	StoryBadgeStyle = makeBadgeStyle("39", "17")   // Blue
	TaskBadgeStyle  = makeBadgeStyle("42", "22")   // Green
	BugBadgeStyle   = makeBadgeStyle("196", "52")  // Red
	IdeaBadgeStyle  = makeBadgeStyle("214", "94")  // Orange
	NoteBadgeStyle  = makeBadgeStyle("248", "237") // Gray

	// Task status badge styles (icon with background for visual distinction)
	TodoBadgeStyle       = makeBadgeStyle("241", "236")                 // Gray
	InProgressBadgeStyle = makeBadgeStyle("214", "94")                  // Orange
	ReviewBadgeStyle     = makeBadgeStyle(string(ColorSecondary), "17") // Blue
	DoneBadgeStyle       = makeBadgeStyle("42", "22")                   // Green
	BlockedBadgeStyle    = makeBadgeStyle("196", "52")                  // Red
	StandbyBadgeStyle    = makeBadgeStyle("141", "53")                  // Violet
	CanceledBadgeStyle   = makeBadgeStyle("241", "236")                 // Gray

	// Container styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	SelectedBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	// Help bar
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight).
			Background(ColorBackground).
			Padding(0, 1)

	// Tree view
	TreeIndent = "  "

	// Form layout
	FormLabelWidth = 11 // Width of label column in form card layouts

	TreeBranchStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	// Progress bar colors
	ProgressFullStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	ProgressEmptyStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	// Scrollbar styles (vertical track on right edge of scrollable lists)
	ScrollbarThumbStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary) // Blue 39

	ScrollbarTrackStyle = lipgloss.NewStyle().
				Foreground(ColorMuted) // Gray 241

	// Badge styles
	ArchivedBadgeStyle = lipgloss.NewStyle().
				Background(ColorWarning).
				Foreground(lipgloss.Color("0")).
				Padding(0, 1)

	// Confirmation prompt style
	ConfirmStyle = lipgloss.NewStyle().
			Background(ColorWarning).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			MarginTop(1)

	// Prefix key popup style (which-key style hints)
	PrefixPopupStyle = lipgloss.NewStyle().
				Background(ColorBackground).
				Foreground(lipgloss.Color("255")).
				Padding(0, 1)

	PrefixKeyStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	PrefixDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Form input styles
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	InputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	FormInputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorBorder)

	FormInputBorderFocusedStyle = lipgloss.NewStyle().
					Border(lipgloss.NormalBorder()).
					BorderForeground(ColorPrimary)

	// Validation error style
	ValidationErrorStyle = lipgloss.NewStyle().
				Foreground(ColorError).
				MarginTop(1)

	// Field label styles
	FieldLabelStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	FieldLabelFocusedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	FieldDescriptionStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Faint(true)

	// Selector option styles
	SelectorOptionStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1)

	SelectorOptionSelectedStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorSelectedFg).
					Padding(0, 1)

	// Success message style
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	// Loading message style (amber/yellow for in-progress operations)
	LoadingStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	// Error status bar style (red for errors)
	ErrorStatusStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	// Search highlight style
	SearchHighlightStyle = lipgloss.NewStyle().
				Background(ColorHighlight).
				Foreground(lipgloss.Color("0"))

	// Help view styles
	HelpViewTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				MarginBottom(1)

	HelpCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginTop(1)

	HelpFilterActiveStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	// Focus mode style
	FocusModeStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			Background(ColorBackground)

	// View mode styles
	ViewModeActiveStyle    = makeViewModeStyle("34")
	ViewModeNextStyle      = makeViewModeStyle("61")
	ViewModeInboxStyle     = makeViewModeStyle(string(ColorSecondary))
	ViewModeWaitingStyle   = makeViewModeStyle("214")
	ViewModeSomedayStyle   = makeViewModeStyle("135")
	ViewModeCompletedStyle = makeViewModeStyle("28")
	ViewModeReviewStyle    = makeViewModeStyle(string(ColorSecondary))
	ViewModeStandbyStyle   = makeViewModeStyle("141")
	ViewModeSnoozedStyle   = makeViewModeStyle("95")
	ViewModeAllStyle       = makeViewModeStyle("60")

	// Filter view styles
	FilterBadgeStyle = lipgloss.NewStyle().
				Background(ColorWarning).
				Foreground(lipgloss.Color("0")).
				Padding(0, 1)

	SavedViewBadgeStyle = lipgloss.NewStyle().
				Background(ColorSecondary).
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Padding(0, 1)

	FilterBadgeInactiveStyle = lipgloss.NewStyle().
					Background(lipgloss.Color("236")). // Dark gray (matches SectionBadgeStyle)
					Foreground(lipgloss.Color("245")). // Gray text
					Padding(0, 1)

	FilterViewBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	FilterItemStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	FilterItemSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorSelectedFg).
				Bold(true)

	FilterValueStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary)

	FilterClearStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	// Task jumper styles
	TaskJumperTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				MarginBottom(1)

	TaskJumperListStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, true, false, false).
				BorderForeground(ColorBorder)

	TaskJumperPreviewStyle = lipgloss.NewStyle().
				Padding(0, 1)

	TaskJumperSelectedStyle = lipgloss.NewStyle().
				Background(ColorSelectedBg).
				Foreground(ColorSelectedFg).
				Bold(true)

	TaskJumperPreviewHeaderStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorPrimary)

	// Due date overview styles
	DueDateOverviewTitleStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorPrimary)

	// GroupHeaderStyle is used for section headers in priority, due date, and similar views.
	GroupHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginTop(1)

	// SectionHeaderStyle is like GroupHeaderStyle but without MarginTop,
	// for views where each entry must be exactly one line (line counting).
	SectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary)

	SelectedStyle = lipgloss.NewStyle().
			Background(ColorSelectedBg).
			Foreground(ColorSelectedFg).
			Bold(true)

	// Convenience style for selected item text (bold + purple fg, no bg)
	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedFg)

	// Selection row styles (unified across all list views)
	SelectionCursorStyle = lipgloss.NewStyle().
				Foreground(ColorSelectedFg).
				Bold(true)

	SelectionRowStyle = lipgloss.NewStyle().
				Background(ColorSelectedBg)

	CursorFlashRowStyle = lipgloss.NewStyle().
				Background(ColorFlashBg)

	// Multi-select styles
	SelectionStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	SelectionCheckStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	SelectionCountStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	// Tag manager styles
	TagManagerBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	TagManagerItemStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	TagManagerItemSelectedStyle = lipgloss.NewStyle().
					Foreground(ColorSelectedFg).
					Bold(true)

	TagManagerCountStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary)

	TagManagerErrorStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	// Section badge style for tree view
	SectionBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Gray text
				Background(lipgloss.Color("236"))  // Dark gray background

	// Children in other sections badge (shown on parent when children are hidden by view filter)
	ChildrenBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")). // Blue (ColorPrimary) - info indicator
				Background(lipgloss.Color("236")) // Dark gray background (matches SectionBadgeStyle)

	// Pinned project style
	PinnedProjectStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	// Pinned project number shortcut style
	PinnedNumberStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	// Unsaved changes indicator style
	UnsavedChangesStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("226")) // Yellow

	// Active task style (yellow to make it pop out)
	ActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")) // Bright yellow

	// Triaged badge style (shown in inbox processor when task will leave inbox)
	TriagedBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorWarning). // Orange
				Italic(true)

	// Priority view styles
	PriorityViewTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	PriorityWaitingHeaderStyle = lipgloss.NewStyle().
					Foreground(ColorMuted).
					Italic(true).
					MarginTop(1)

	// Suggestion styles (for Priority View smart suggestions)
	SuggestionBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("226")). // Bright yellow
				Padding(0, 1)

	SuggestionHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("226")). // Bright yellow
				Bold(true)

	SuggestionReasonStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true)

	SuggestionStarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("226")). // Bright yellow (matches ActiveStyle)
				Bold(true)

	// Priority level styles (for priority picker and palette)
	Priority1Style = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	Priority2Style = makeColorStyle(ColorWarning)
	Priority3Style = makeColorStyle(ColorPrimary)
	Priority4Style = makeColorStyle(ColorMuted)
	Priority5Style = makeColorStyle(ColorMuted).Faint(true)

	// Common inline styles used across multiple files
	MatchHighlightStyle = lipgloss.NewStyle().Foreground(ColorHighlight)
	TagInlineStyle      = lipgloss.NewStyle().Foreground(ColorSecondary)
	PriorityInlineStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	CheckmarkStyle      = lipgloss.NewStyle().Foreground(ColorSuccess)
	WarningIconStyle    = lipgloss.NewStyle().Foreground(ColorWarning)
	ErrorIconStyle      = lipgloss.NewStyle().Foreground(ColorError)
	PurpleStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("135"))
	OrangeStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

// GetStatusStyle returns the appropriate style for a task status.
func GetStatusStyle(status string) lipgloss.Style {
	switch status {
	case "todo":
		return TodoStyle
	case "in_progress":
		return InProgressStyle
	case "review":
		return ReviewStyle
	case "standby":
		return StandbyStyle
	case "done":
		return DoneStyle
	case "blocked":
		return BlockedStyle
	case "canceled":
		return CanceledStyle
	default:
		return NormalStyle
	}
}

// GetPriorityStyle returns the appropriate style for a priority level.
func GetPriorityStyle(priority int) lipgloss.Style {
	switch priority {
	case 1:
		return Priority1Style
	case 2:
		return Priority2Style
	case 3:
		return Priority3Style
	case 4:
		return Priority4Style
	case 5:
		return Priority5Style
	default:
		return NormalStyle
	}
}

// GetTypeStyle returns the appropriate style for a task type.
func GetTypeStyle(taskType string) lipgloss.Style {
	switch taskType {
	case "epic":
		return EpicStyle
	case "story":
		return StoryStyle
	case "task":
		return TaskStyle
	case "bug":
		return BugStyle
	case "idea":
		return IdeaStyle
	case "note":
		return NoteStyle
	default:
		return NormalStyle
	}
}

// GetTypeBadge returns the task type icon rendered as a styled badge with background.
func GetTypeBadge(taskType string, icon string) string {
	switch taskType {
	case "epic":
		return EpicBadgeStyle.Render(icon)
	case "story":
		return StoryBadgeStyle.Render(icon)
	case "task":
		return TaskBadgeStyle.Render(icon)
	case "bug":
		return BugBadgeStyle.Render(icon)
	case "idea":
		return IdeaBadgeStyle.Render(icon)
	case "note":
		return NoteBadgeStyle.Render(icon)
	default:
		return NormalStyle.Render(icon)
	}
}

// GetStatusBadge returns the task status icon rendered as a styled badge with background.
func GetStatusBadge(status string, icon string) string {
	switch status {
	case "todo":
		return TodoBadgeStyle.Render(icon)
	case "in_progress":
		return InProgressBadgeStyle.Render(icon)
	case "review":
		return ReviewBadgeStyle.Render(icon)
	case "standby":
		return StandbyBadgeStyle.Render(icon)
	case "done":
		return DoneBadgeStyle.Render(icon)
	case "blocked":
		return BlockedBadgeStyle.Render(icon)
	case "canceled":
		return CanceledBadgeStyle.Render(icon)
	default:
		return NormalStyle.Render(icon)
	}
}

// RenderProgressBar renders a simple progress bar.
func RenderProgressBar(percent float64, width int) string {
	filled := int(float64(width) * percent / 100)
	empty := width - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += ProgressFullStyle.Render("█")
	}
	for i := 0; i < empty; i++ {
		bar += ProgressEmptyStyle.Render("░")
	}

	return bar
}

// SectionColorEntry represents a color in the section color palette.
type SectionColorEntry struct {
	Name  string
	Color lipgloss.Color
}

// SectionColorPalette defines the available colors for sections.
var SectionColorPalette = []SectionColorEntry{
	{"gray", lipgloss.Color("245")},
	{"red", lipgloss.Color("196")},
	{"orange", lipgloss.Color("208")},
	{"yellow", lipgloss.Color("220")},
	{"green", lipgloss.Color("42")},
	{"cyan", lipgloss.Color("44")},
	{"blue", lipgloss.Color("33")},
	{"purple", lipgloss.Color("135")},
	{"magenta", lipgloss.Color("198")},
	{"white", lipgloss.Color("255")},
}

// GetViewModeStyleForSection returns the ViewMode style for a section name.
// This maps section strings to their corresponding ViewMode styles, providing
// consistent styling between the view mode indicator and kanban column headers.
func GetViewModeStyleForSection(section string) lipgloss.Style {
	switch section {
	case "": // Active section
		return ViewModeActiveStyle
	case "Next":
		return ViewModeNextStyle
	case "Inbox":
		return ViewModeInboxStyle
	case "Waiting":
		return ViewModeWaitingStyle
	case "Someday":
		return ViewModeSomedayStyle
	case "Review":
		return ViewModeReviewStyle
	case "Standby":
		return ViewModeStandbyStyle
	default:
		return ViewModeActiveStyle
	}
}

// GetSectionColorStyle returns a style for a section badge given a color name.
func GetSectionColorStyle(colorName string) lipgloss.Style {
	for _, pc := range SectionColorPalette {
		if pc.Name == colorName {
			return lipgloss.NewStyle().
				Foreground(pc.Color).
				Background(lipgloss.Color("236")) // Dark background
		}
	}
	// Default to gray
	return SectionBadgeStyle
}

// GetSectionColorIndex returns the index of a color name in the palette.
func GetSectionColorIndex(colorName string) int {
	for i, pc := range SectionColorPalette {
		if pc.Name == colorName {
			return i
		}
	}
	return 0 // Default to gray
}

// GetSectionColorByIndex returns the color name at the given palette index.
func GetSectionColorByIndex(index int) string {
	if index < 0 || index >= len(SectionColorPalette) {
		return "gray"
	}
	return SectionColorPalette[index].Name
}

// GetSectionColorValue returns the lipgloss.Color for a section color name.
// Returns empty string if the color name is empty or not found.
func GetSectionColorValue(colorName string) lipgloss.Color {
	if colorName == "" {
		return ""
	}
	for _, pc := range SectionColorPalette {
		if pc.Name == colorName {
			return pc.Color
		}
	}
	return ""
}

// Sidebar styles
var (
	SidebarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1)

	SidebarHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	SidebarSectionHeaderStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorSecondary).
					Underline(true)

	TagStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Background(ColorBackground).
			Padding(0, 1)

	SidebarFocusBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("226")). // Bright yellow like ActiveStyle
				Bold(true)

	// Sidebar card styles (compact boxes for sidebar sections)
	SidebarCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1)

	SidebarCardFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	SidebarCardTitleStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	// Dashboard card styles (full-width cards for dashboard sections)
	DashboardCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1)

	DashboardCardTitleStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true)
)

// Additional shared styles for common UI patterns
var (
	// Selector and list styles
	SelectorTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				MarginBottom(1)

	SelectorSearchStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginBottom(1)

	SelectorCountStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				MarginBottom(1)

	SelectorNoResultsStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true).
				MarginTop(1).
				MarginBottom(1)

	SelectorBaseStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(ColorBorder)

	// Empty/error state styles
	EmptyErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorError)

	// Separator for split-pane views
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Category badge styles for command palette
	CategoryCommandStyle     = lipgloss.NewStyle().Foreground(ColorPrimary)
	CategoryTaskStyle        = lipgloss.NewStyle().Foreground(ColorSuccess)
	CategoryProjectStyle     = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryTagStyle         = lipgloss.NewStyle().Foreground(ColorWarning)
	CategorySectionStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("135")) // Purple
	CategorySavedSearchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	CategorySavedViewStyle   = lipgloss.NewStyle().Foreground(ColorSecondary)        // Blue (matches saved view badge)
	CategoryNoteStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("108")) // Sage green
	CategoryViewStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))  // Sky blue

	// Selection indicators
	SelectedCheckboxStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	PinIndicatorStyle     = lipgloss.NewStyle().Foreground(ColorWarning)

	// Due date urgency styles
	DueThisWeekStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	// Header style (bold, no color)
	HeaderBoldStyle = lipgloss.NewStyle().Bold(true)

	// Preview style (warning color for bulk edit, etc.)
	PreviewWarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	// Overlay split-pane separator (vertical line between list and preview)
	OverlaySplitStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)

	// Overlay panel styles (used by kanban display settings, score profile manager, etc.)
	OverlayTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	OverlaySelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	OverlayDetailStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	OverlayBoxStyle      = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("99")).
				Padding(1, 2)

	// Diff styles
	DiffAddStyle     = lipgloss.NewStyle().Foreground(ColorSuccess)
	DiffRemoveStyle  = lipgloss.NewStyle().Foreground(ColorError)
	DiffContextStyle = lipgloss.NewStyle().Foreground(ColorDiffContext)
)

// FocusIndicatorStyle is used for the ▐ bar that shows which tab row has focus.
var FocusIndicatorStyle = lipgloss.NewStyle().Foreground(ColorPrimary)

// FocusIndicator is the pre-rendered ▐ bar used to indicate focused tab rows.
var FocusIndicator = FocusIndicatorStyle.Render("▐")

// PinnedMarker is a green dot used to indicate pinned tasks in list views.
var PinnedMarker = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("·")

// RenderSelectableRow applies consistent selection styling to list items.
// When selected=true, adds cursor prefix and row background that extends to full width.
// When active=true, adds an asterisk indicator after the cursor position.
// When pinned=true, adds a green dot indicator.
// The content should already have its internal styling (badges, etc).
// RenderCursorPrefix returns the 3-character cursor prefix string based on
// selected/active/pinned state. Always exactly 3 display characters wide.
func RenderCursorPrefix(selected, active, pinned bool) string {
	activeMarker := ActiveStyle.Bold(true).Render("*")
	switch {
	case selected && active && pinned:
		return SelectionCursorStyle.Render(">") + activeMarker + PinnedMarker
	case selected && active:
		return SelectionCursorStyle.Render(">") + activeMarker + " "
	case selected && pinned:
		return SelectionCursorStyle.Render(">") + " " + PinnedMarker
	case selected:
		return SelectionCursorStyle.Render(">") + "  "
	case active && pinned:
		return " " + activeMarker + PinnedMarker
	case active:
		return " " + activeMarker + " "
	case pinned:
		return "  " + PinnedMarker
	default:
		return "   "
	}
}

// RenderSelectableRow renders a row with cursor prefix and optional full-width
// selection highlight. Uses RenderCursorPrefix for the 3-char prefix, then
// applies SelectionRowStyle with the given width when selected.
func RenderSelectableRow(selected bool, active bool, pinned bool, flash bool, content string, width int) string {
	row := RenderCursorPrefix(selected, active, pinned) + content
	if selected {
		style := SelectionRowStyle
		if flash {
			style = CursorFlashRowStyle
		}
		return style.Width(width).Render(row)
	}
	return row
}

// Hyperlink creates a clickable OSC 8 hyperlink in supported terminals.
// The displayed text can differ from the actual URL, so you can show truncated
// text while linking to the full URL.
// Format: \033]8;;URL\033\\Display Text\033]8;;\033\\
func Hyperlink(url, displayText string) string {
	return "\033]8;;" + url + "\033\\" + displayText + "\033]8;;\033\\"
}
