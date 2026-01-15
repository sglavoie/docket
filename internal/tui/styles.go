package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	ColorPrimary    = lipgloss.Color("39")  // Blue
	ColorSecondary  = lipgloss.Color("33")  // Cyan
	ColorSuccess    = lipgloss.Color("42")  // Green
	ColorWarning    = lipgloss.Color("214") // Orange
	ColorError      = lipgloss.Color("196") // Red
	ColorMuted      = lipgloss.Color("241") // Gray
	ColorHighlight  = lipgloss.Color("229") // Light yellow
	ColorBackground = lipgloss.Color("236") // Dark gray
	ColorBorder     = lipgloss.Color("240") // Border gray
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

	DoneStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	BlockedStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	CancelledStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Strikethrough(true)

	OverdueStyle = lipgloss.NewStyle().
			Foreground(ColorError)

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

	TreeBranchStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	// Progress bar colors
	ProgressFullStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	ProgressEmptyStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

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

	// Selector option styles
	SelectorOptionStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1)

	SelectorOptionSelectedStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorPrimary).
					Padding(0, 1)

	// Success message style
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

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
			Foreground(lipgloss.Color("33")).
			Bold(true).
			Background(lipgloss.Color("236"))

	// View mode styles
	ViewModeActiveStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("34")).
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Padding(0, 1)

	ViewModeInboxStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("33")).
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Padding(0, 1)

	ViewModeBacklogStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("241")).
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Padding(0, 1)

	ViewModeCompletedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("28")).
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Padding(0, 1)

	// Filter view styles
	FilterBadgeStyle = lipgloss.NewStyle().
				Background(ColorWarning).
				Foreground(lipgloss.Color("0")).
				Padding(0, 1)

	FilterViewBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	FilterItemStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	FilterItemSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
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
				Background(ColorBackground).
				Foreground(ColorPrimary).
				Bold(true)

	TaskJumperPreviewHeaderStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorPrimary)

	// Due date overview styles
	DueDateOverviewTitleStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorPrimary)

	DueDateGroupHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginTop(1)

	SelectedStyle = lipgloss.NewStyle().
			Background(ColorBackground).
			Foreground(ColorPrimary).
			Bold(true)

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
					Foreground(ColorPrimary).
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

	// Priority view styles
	PriorityViewTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	PriorityGroupHeaderStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorSecondary).
					MarginTop(1)

	PriorityWaitingHeaderStyle = lipgloss.NewStyle().
					Foreground(ColorMuted).
					Italic(true).
					MarginTop(1)
)

// GetStatusStyle returns the appropriate style for a task status.
func GetStatusStyle(status string) lipgloss.Style {
	switch status {
	case "todo":
		return TodoStyle
	case "in_progress":
		return InProgressStyle
	case "done":
		return DoneStyle
	case "blocked":
		return BlockedStyle
	case "cancelled":
		return CancelledStyle
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
)
